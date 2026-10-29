package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func core02Send(h *harnessAuth, method, path string, body any, token string, headers map[string]string) responsUji {
	h.t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	result := responsUji{Status: rec.Code, Rec: rec}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &result.Body); err != nil {
			h.t.Fatalf("invalid JSON: %s: %v", rec.Body.String(), err)
		}
	}
	return result
}

func core02Session(h *harnessAuth, token, classID string, late *time.Time) string {
	h.t.Helper()
	body := map[string]any{"class_id": classID, "title": "Sesi Sintetis", "closes_at": h.clk.Now().Add(5 * time.Minute)}
	if late != nil {
		body["late_after"] = *late
	}
	return h.harapSukses(h.kirim(http.MethodPost, "/api/v1/sessions", body, token), http.StatusCreated)["id"].(string)
}

func core02Manual(person, status string) map[string]any {
	return map[string]any{"person_id": person, "status": status, "reason": "Koreksi sintetis privat"}
}

func TestCORE02HTTPManualLifecycleReplayAndPrivacy(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("admin.core02@kampus.example")
	studentID, student := h.undangDanMasuk(admin.Access, "student.core02@kampus.example", "Mahasiswa Sintetis", "mahasiswa")
	_, unlinked := h.undangDanMasuk(admin.Access, "unlinked.core02@kampus.example", "Tanpa Tautan", "mahasiswa")
	person := corePerson(h, admin.Access, "CORE02-PRIVATE")
	missing := corePerson(h, admin.Access, "CORE02-ABSENT")
	classID := coreClass(h, admin.Access, "CORE02-A")
	for _, id := range []string{person, missing} {
		h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+classID+"/members/"+id, nil, admin.Access), http.StatusCreated)
	}
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/people/"+person+"/account-link", map[string]any{"user_id": studentID}, admin.Access), http.StatusOK)
	if n := h.hitung("SELECT COUNT(*) FROM consent_records"); n != 0 {
		t.Fatalf("unexpected consent fixtures=%d", n)
	}
	sessionID := core02Session(h, admin.Access, classID, nil)
	path := "/api/v1/sessions/" + sessionID
	checkins := path + "/manual-checkins"
	attendance := path + "/attendance?include=counts"
	pending := h.harapSuksesLarik(h.kirim(http.MethodGet, attendance, nil, admin.Access), http.StatusOK)
	for _, row := range pending {
		if row.(map[string]any)["status"] != "pending" {
			t.Fatalf("draft must be pending: %v", pending)
		}
	}
	h.harapGalat(h.kirim(http.MethodPost, checkins, core02Manual(person, "present"), admin.Access), http.StatusUnprocessableEntity, "SESSION_NOT_OPEN")
	opened := h.harapSukses(h.kirim(http.MethodPost, path+"/open", nil, admin.Access), http.StatusOK)
	h.harapGalat(h.kirim(http.MethodPost, path+"/open", nil, admin.Access), http.StatusConflict, "SESSION_ALREADY_OPEN")
	initial := h.kirim(http.MethodGet, attendance, nil, admin.Access)
	h.harapSuksesLarik(initial, http.StatusOK)
	tag := initial.Rec.Header().Get("ETag")
	if tag == "" {
		t.Fatal("missing ETag")
	}
	cached := core02Send(h, http.MethodGet, attendance, nil, admin.Access, map[string]string{"If-None-Match": tag})
	if cached.Status != http.StatusNotModified || cached.Rec.Body.Len() != 0 {
		t.Fatalf("cached response: %d %s", cached.Status, cached.Rec.Body.String())
	}
	body := core02Manual(person, "present")
	headers := map[string]string{"Idempotency-Key": "manual-core02-key"}
	first := h.harapSukses(core02Send(h, http.MethodPost, checkins, body, admin.Access, headers), http.StatusCreated)
	replay := h.harapSukses(core02Send(h, http.MethodPost, checkins, body, admin.Access, headers), http.StatusCreated)
	firstJSON, _ := json.Marshal(first)
	replayJSON, _ := json.Marshal(replay)
	if !bytes.Equal(firstJSON, replayJSON) {
		t.Fatalf("replay changed: %s / %s", firstJSON, replayJSON)
	}
	h.harapGalat(h.kirim(http.MethodPost, checkins, body, admin.Access), http.StatusConflict, "ALREADY_CHECKED_IN")
	changed := core02Manual(person, "excused")
	h.harapGalat(core02Send(h, http.MethodPost, checkins, changed, admin.Access, headers), http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	updated := core02Send(h, http.MethodGet, attendance, nil, admin.Access, map[string]string{"If-None-Match": tag})
	h.harapSuksesLarik(updated, http.StatusOK)
	if updated.Rec.Header().Get("ETag") == tag {
		t.Fatal("attendance mutation retained ETag")
	}
	// No account binding or consent is needed for an authorized manual alternative.
	h.clk.Advance(time.Second)
	h.harapSukses(h.kirim(http.MethodPost, path+"/close", map[string]any{"reason": "Penutupan sintetis privat"}, admin.Access), http.StatusOK)
	h.harapGalat(h.kirim(http.MethodPost, path+"/close", nil, admin.Access), http.StatusConflict, "SESSION_CLOSED")
	closed := h.kirim(http.MethodGet, attendance, nil, admin.Access)
	rows := h.harapSuksesLarik(closed, http.StatusOK)
	if !core02HasStatus(rows, missing, "absent") || closed.Body["counts"].(map[string]any)["absent"] != float64(1) {
		t.Fatalf("closed projection: %v", closed.Body)
	}
	h.harapGalat(h.kirim(http.MethodPost, checkins, core02Manual(missing, "present"), admin.Access), http.StatusConflict, "SESSION_CLOSED")
	correction := core02Manual(person, "excused")
	correction["checked_in_at"] = opened["started_at"]
	h.harapSukses(h.kirim(http.MethodPost, checkins, correction, admin.Access), http.StatusOK)
	correction["person_id"] = missing
	h.harapSukses(h.kirim(http.MethodPost, checkins, correction, admin.Access), http.StatusCreated)
	correction["person_id"] = person
	h.harapSukses(h.kirim(http.MethodPost, checkins, correction, admin.Access), http.StatusOK)
	replay = h.harapSukses(core02Send(h, http.MethodPost, checkins, body, admin.Access, headers), http.StatusCreated)
	replayJSON, _ = json.Marshal(replay)
	if !bytes.Equal(firstJSON, replayJSON) {
		t.Fatalf("later correction rewrote replay snapshot: %s", replayJSON)
	}
	history := h.harapSuksesLarik(h.kirim(http.MethodGet, "/api/v1/me/attendance", nil, student.Access), http.StatusOK)
	if len(history) != 1 || history[0].(map[string]any)["status"] != "excused" {
		t.Fatalf("self history=%v", history)
	}
	for _, row := range history {
		for _, forbidden := range []string{"person_id", "external_code", "display_name", "created_by", "embedding", "face_template"} {
			if _, ok := row.(map[string]any)[forbidden]; ok {
				t.Fatalf("private field %s in self history", forbidden)
			}
		}
	}
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/me/attendance", nil, unlinked.Access), http.StatusConflict, codeAccountNotLinked)
	for _, selector := range []string{"person_id=" + missing, "user_id=" + studentID, "person_id=", "account_user_id=" + studentID} {
		h.harapGalat(h.kirim(http.MethodGet, "/api/v1/me/attendance?"+selector, nil, student.Access), http.StatusBadRequest, codeValidationFailed)
	}
	manual := h.harapSuksesLarik(h.kirim(http.MethodGet, checkins, nil, admin.Access), http.StatusOK)
	if len(manual) != 2 {
		t.Fatalf("manual count=%v", manual)
	}
	for _, metadata := range h.metadataAudit() {
		if strings.Contains(metadata, "Koreksi sintetis privat") || strings.Contains(metadata, "Penutupan sintetis privat") || strings.Contains(metadata, "CORE02-PRIVATE") {
			t.Fatalf("audit privacy: %s", metadata)
		}
	}
	logs := h.logs.String()
	for _, forbidden := range []string{"Koreksi sintetis privat", "Penutupan sintetis privat", "CORE02-PRIVATE", "manual-core02-key"} {
		if strings.Contains(logs, forbidden) {
			t.Fatalf("log leaked %s", forbidden)
		}
	}
	for _, table := range []string{"face_enrollments", "consent_records"} {
		if n := h.hitung("SELECT COUNT(*) FROM " + table); n != 0 {
			t.Fatalf("manual path touched %s=%d", table, n)
		}
	}
	if n := h.hitung("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('face_templates','recognition_attempts')"); n != 0 {
		t.Fatalf("unexpected biometric storage=%d", n)
	}
}

func core02HasStatus(rows []any, person, status string) bool {
	for _, row := range rows {
		object := row.(map[string]any)
		if object["person_id"] == person && object["status"] == status {
			return true
		}
	}
	return false
}

func TestCORE02HTTPRolesScopeAndCacheAuthorization(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("admin.scope02@kampus.example")
	lecturerID, lecturer := h.undangDanMasuk(admin.Access, "lecturer.scope02@kampus.example", "Dosen Sintetis", "dosen")
	_, student := h.undangDanMasuk(admin.Access, "student.scope02@kampus.example", "Mahasiswa Sintetis", "mahasiswa")
	own := coreClass(h, admin.Access, "CORE02-OWN")
	other := coreClass(h, admin.Access, "CORE02-OTHER")
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+own+"/dosen/"+lecturerID, nil, admin.Access), http.StatusCreated)
	id := core02Session(h, lecturer.Access, own, nil)
	otherID := core02Session(h, admin.Access, other, nil)
	person := corePerson(h, admin.Access, "CORE02-SCOPE")
	for _, route := range []string{"", "/open", "/close", "/manual-checkins", "/attendance"} {
		method := http.MethodGet
		var body any
		if route == "/open" || route == "/close" {
			method = http.MethodPost
		}
		h.harapGalat(h.kirim(method, "/api/v1/sessions/"+otherID+route, body, lecturer.Access), http.StatusNotFound, codeNotFound)
		h.harapGalat(h.kirim(method, "/api/v1/sessions/"+id+route, body, student.Access), http.StatusForbidden, codeForbiddenRole)
	}
	h.harapGalat(h.kirim(http.MethodPost, "/api/v1/sessions/"+otherID+"/manual-checkins", core02Manual(person, "present"), lecturer.Access), http.StatusNotFound, codeNotFound)
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/sessions?class_id="+other, nil, lecturer.Access), http.StatusNotFound, codeNotFound)
	rows := h.harapSuksesLarik(h.kirim(http.MethodGet, "/api/v1/sessions", nil, lecturer.Access), http.StatusOK)
	if len(rows) != 1 || !cariNilaiLarik(rows, "id", id) {
		t.Fatalf("scope sessions=%v", rows)
	}
	for _, token := range []string{admin.Access, lecturer.Access} {
		h.harapGalat(h.kirim(http.MethodGet, "/api/v1/me/attendance", nil, token), http.StatusForbidden, codeForbiddenRole)
	}
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/sessions", nil, ""), http.StatusUnauthorized, codeUnauthenticated)
	path := "/api/v1/sessions/" + id + "/attendance"
	authorized := h.kirim(http.MethodGet, path, nil, lecturer.Access)
	h.harapSuksesLarik(authorized, http.StatusOK)
	headers := map[string]string{"If-None-Match": authorized.Rec.Header().Get("ETag")}
	if removed := h.kirim(http.MethodDelete, "/api/v1/classes/"+own+"/dosen/"+lecturerID, nil, admin.Access); removed.Status != http.StatusNoContent {
		t.Fatalf("remove assignment: %s", removed.Rec.Body.String())
	}
	h.harapGalat(core02Send(h, http.MethodGet, path, nil, lecturer.Access, headers), http.StatusNotFound, codeNotFound)
	h.harapGalat(core02Send(h, http.MethodGet, path, nil, student.Access, headers), http.StatusForbidden, codeForbiddenRole)
}

func TestCORE02HTTPTimeThresholdsAndStrictParsing(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("admin.time02@kampus.example")
	person := corePerson(h, admin.Access, "CORE02-TIME")
	other := corePerson(h, admin.Access, "CORE02-NONMEMBER")
	classID := coreClass(h, admin.Access, "CORE02-TIME")
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+classID+"/members/"+person, nil, admin.Access), http.StatusCreated)
	late := h.clk.Now().Add(2 * time.Second)
	id := core02Session(h, admin.Access, classID, &late)
	path := "/api/v1/sessions/" + id
	h.harapSukses(h.kirim(http.MethodPost, path+"/open", nil, admin.Access), http.StatusOK)
	h.clk.Advance(2 * time.Second)
	body := core02Manual(person, "present")
	first := h.harapSukses(h.kirim(http.MethodPost, path+"/manual-checkins", body, admin.Access), http.StatusCreated)
	if first["status"] != "present" {
		t.Fatalf("threshold equality=%v", first)
	}
	h.clk.Advance(time.Second)
	body["checked_in_at"] = h.clk.Now()
	h.harapGalat(h.kirim(http.MethodPost, path+"/manual-checkins", body, admin.Access), http.StatusUnprocessableEntity, codeValidationFailed)
	body["status"] = "late"
	h.harapSukses(h.kirim(http.MethodPost, path+"/manual-checkins", body, admin.Access), http.StatusOK)
	for _, when := range []time.Time{late.Add(-3 * time.Second), h.clk.Now().Add(time.Second), h.clk.Now().Add(5 * time.Minute)} {
		body["checked_in_at"] = when
		h.harapGalat(h.kirim(http.MethodPost, path+"/manual-checkins", body, admin.Access), http.StatusUnprocessableEntity, codeValidationFailed)
	}
	h.harapGalat(h.kirim(http.MethodPost, path+"/manual-checkins", core02Manual(other, "late"), admin.Access), http.StatusUnprocessableEntity, "CLASS_MISMATCH")
	for _, body := range []map[string]any{{"person_id": person, "status": "unknown", "reason": "valid"}, {"person_id": person, "status": "present", "reason": "valid", "checked_in_at": "not-time"}, {"person_id": person, "status": "present", "reason": "valid", "user_id": "forged"}} {
		h.harapGalat(h.kirim(http.MethodPost, path+"/manual-checkins", body, admin.Access), http.StatusBadRequest, codeValidationFailed)
	}
	h.harapGalat(h.kirim(http.MethodPost, path+"/manual-checkins", map[string]any{"person_id": person, "status": "late", "reason": " "}, admin.Access), http.StatusBadRequest, "REASON_REQUIRED")
	for _, key := range []string{"tiny", strings.Repeat("a", 129), "invalid\tkey"} {
		h.harapGalat(core02Send(h, http.MethodPost, path+"/manual-checkins", core02Manual(person, "late"), admin.Access, map[string]string{"Idempotency-Key": key}), http.StatusBadRequest, codeValidationFailed)
	}
	for _, query := range []string{"status=cancelled", "from=bad", "from=2026-01-02T00:00:00Z&to=2026-01-01T00:00:00Z", "limit=0", "limit=no", "cursor=bad", "status=open&status=closed", "person_id=forged"} {
		h.harapGalat(h.kirim(http.MethodGet, "/api/v1/sessions?"+query, nil, admin.Access), http.StatusBadRequest, codeValidationFailed)
	}
	h.harapGalat(h.kirim(http.MethodGet, path+"/attendance?include=biometrics", nil, admin.Access), http.StatusBadRequest, codeValidationFailed)
	h.harapGalat(h.kirim(http.MethodPost, "/api/v1/sessions", map[string]any{"class_id": classID, "closes_at": "not-time"}, admin.Access), http.StatusBadRequest, codeValidationFailed)
}

func TestCORE02HTTPSessionIdempotencyAndPagination(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("admin.page02@kampus.example")
	classID := coreClass(h, admin.Access, "CORE02-PAGE")
	body := map[string]any{"class_id": classID, "title": "Sesi replay privat", "closes_at": h.clk.Now().Add(5 * time.Minute)}
	headers := map[string]string{"Idempotency-Key": "create-session-core02"}
	first := h.harapSukses(core02Send(h, http.MethodPost, "/api/v1/sessions", body, admin.Access, headers), http.StatusCreated)
	replay := h.harapSukses(core02Send(h, http.MethodPost, "/api/v1/sessions", body, admin.Access, headers), http.StatusCreated)
	if first["id"] != replay["id"] || first["created_at"] != replay["created_at"] {
		t.Fatalf("create replay=%v / %v", first, replay)
	}
	body["title"] = "Altered"
	h.harapGalat(core02Send(h, http.MethodPost, "/api/v1/sessions", body, admin.Access, headers), http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	second := core02Session(h, admin.Access, classID, nil)
	page := h.kirim(http.MethodGet, "/api/v1/sessions?limit=1", nil, admin.Access)
	rows := h.harapSuksesLarik(page, http.StatusOK)
	metadata := page.Body["page"].(map[string]any)
	if len(rows) != 1 || metadata["has_more"] != true {
		t.Fatalf("first page=%v", page.Body)
	}
	cursor := metadata["next_cursor"].(string)
	next := h.harapSuksesLarik(h.kirim(http.MethodGet, "/api/v1/sessions?limit=1&cursor="+cursor, nil, admin.Access), http.StatusOK)
	if len(next) != 1 || next[0].(map[string]any)["id"] == rows[0].(map[string]any)["id"] {
		t.Fatalf("page replay=%v / %v", rows, next)
	}
	if !cariNilaiLarik(append(rows, next...), "id", second) {
		t.Fatal("missing second session")
	}
	attendance := "/api/v1/sessions/" + first["id"].(string) + "/attendance"
	h.harapGalat(h.kirim(http.MethodGet, attendance+"?cursor="+cursor, nil, admin.Access), http.StatusBadRequest, codeValidationFailed)
	for _, code := range []string{"CORE02-P1", "CORE02-P2"} {
		person := corePerson(h, admin.Access, code)
		h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+classID+"/members/"+person, nil, admin.Access), http.StatusCreated)
	}
	attendees := h.kirim(http.MethodGet, attendance+"?limit=1&include=counts", nil, admin.Access)
	firstRows := h.harapSuksesLarik(attendees, http.StatusOK)
	attendanceCursor := attendees.Body["page"].(map[string]any)["next_cursor"].(string)
	secondRows := h.harapSuksesLarik(h.kirim(http.MethodGet, attendance+"?limit=1&cursor="+attendanceCursor, nil, admin.Access), http.StatusOK)
	if len(firstRows) != 1 || len(secondRows) != 1 || firstRows[0].(map[string]any)["person_id"] == secondRows[0].(map[string]any)["person_id"] {
		t.Fatalf("attendance pagination=%v / %v", firstRows, secondRows)
	}
	differentQuery := core02Send(h, http.MethodGet, attendance+"?limit=2&include=counts", nil, admin.Access, map[string]string{"If-None-Match": attendees.Rec.Header().Get("ETag")})
	h.harapSuksesLarik(differentQuery, http.StatusOK)
}

func TestCORE02HTTPExpiredWindowAndOwnPagination(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("admin.own02@kampus.example")
	studentID, student := h.undangDanMasuk(admin.Access, "student.own02@kampus.example", "Mahasiswa Sintetis", "mahasiswa")
	otherID, other := h.undangDanMasuk(admin.Access, "other.own02@kampus.example", "Mahasiswa Lain", "mahasiswa")
	person := corePerson(h, admin.Access, "CORE02-OWN-1")
	another := corePerson(h, admin.Access, "CORE02-OWN-2")
	classID := coreClass(h, admin.Access, "CORE02-OWN-HISTORY")
	for _, pair := range [][2]string{{person, studentID}, {another, otherID}} {
		h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+classID+"/members/"+pair[0], nil, admin.Access), http.StatusCreated)
		h.harapSukses(h.kirim(http.MethodPut, "/api/v1/people/"+pair[0]+"/account-link", map[string]any{"user_id": pair[1]}, admin.Access), http.StatusOK)
	}
	for i := range 2 {
		body := map[string]any{"class_id": classID, "closes_at": h.clk.Now().Add(2 * time.Second)}
		sessionID := h.harapSukses(h.kirim(http.MethodPost, "/api/v1/sessions", body, admin.Access), http.StatusCreated)["id"].(string)
		path := "/api/v1/sessions/" + sessionID
		opened := h.harapSukses(h.kirim(http.MethodPost, path+"/open", nil, admin.Access), http.StatusOK)
		h.harapSukses(h.kirim(http.MethodPost, path+"/manual-checkins", core02Manual(person, "present"), admin.Access), http.StatusCreated)
		h.clk.Advance(2 * time.Second)
		// Equal closes_at is already outside the implicit check-in window, but
		// absence is not projected until an authorized close is committed.
		h.harapGalat(h.kirim(http.MethodPost, path+"/manual-checkins", core02Manual(another, "present"), admin.Access), http.StatusConflict, "SESSION_CLOSED")
		pending := h.harapSuksesLarik(h.kirim(http.MethodGet, path+"/attendance", nil, admin.Access), http.StatusOK)
		if !core02HasStatus(pending, another, "pending") {
			t.Fatalf("elapsed open projected absent: %v", pending)
		}
		explicit := core02Manual(another, "present")
		explicit["checked_in_at"] = body["closes_at"]
		h.harapGalat(h.kirim(http.MethodPost, path+"/manual-checkins", explicit, admin.Access), http.StatusUnprocessableEntity, codeValidationFailed)
		h.harapSukses(h.kirim(http.MethodPost, path+"/close", nil, admin.Access), http.StatusOK)
		if i == 0 {
			explicit["checked_in_at"] = opened["started_at"]
			h.harapSukses(h.kirim(http.MethodPost, path+"/manual-checkins", explicit, admin.Access), http.StatusCreated)
		}
		h.clk.Advance(time.Second)
	}
	first := h.kirim(http.MethodGet, "/api/v1/me/attendance?limit=1", nil, student.Access)
	firstRows := h.harapSuksesLarik(first, http.StatusOK)
	page := first.Body["page"].(map[string]any)
	if len(firstRows) != 1 || page["has_more"] != true {
		t.Fatalf("own first page=%v", first.Body)
	}
	second := h.harapSuksesLarik(h.kirim(http.MethodGet, "/api/v1/me/attendance?limit=1&cursor="+page["next_cursor"].(string), nil, student.Access), http.StatusOK)
	if len(second) != 1 || second[0].(map[string]any)["session_id"] == firstRows[0].(map[string]any)["session_id"] {
		t.Fatalf("own cursor repeated: %v / %v", firstRows, second)
	}
	otherRows := h.harapSuksesLarik(h.kirim(http.MethodGet, "/api/v1/me/attendance", nil, other.Access), http.StatusOK)
	if len(otherRows) != 2 {
		t.Fatalf("other history=%v", otherRows)
	}
	statuses := map[string]int{}
	for _, row := range otherRows {
		statuses[row.(map[string]any)["status"].(string)]++
	}
	if statuses["present"] != 1 || statuses["absent"] != 1 {
		t.Fatalf("ownership projection=%v", otherRows)
	}
	h.execSQL("UPDATE people SET identity_verified_at=NULL WHERE id=?", person)
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/me/attendance", nil, student.Access), http.StatusConflict, codeAccountNotLinked)
}

func TestCORE02HTTPReplayReauthorizesCurrentScope(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("admin.reauth02@kampus.example")
	lecturerID, lecturer := h.undangDanMasuk(admin.Access, "lecturer.reauth02@kampus.example", "Dosen Sintetis", "dosen")
	classID := coreClass(h, admin.Access, "CORE02-REAUTHORIZE")
	person := corePerson(h, admin.Access, "CORE02-REAUTHORIZE")
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+classID+"/dosen/"+lecturerID, nil, admin.Access), http.StatusCreated)
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+classID+"/members/"+person, nil, admin.Access), http.StatusCreated)
	create := map[string]any{"class_id": classID, "closes_at": h.clk.Now().Add(time.Minute)}
	createHeaders := map[string]string{"Idempotency-Key": "scope-create-core02"}
	sessionID := h.harapSukses(core02Send(h, http.MethodPost, "/api/v1/sessions", create, lecturer.Access, createHeaders), http.StatusCreated)["id"].(string)
	path := "/api/v1/sessions/" + sessionID
	h.harapSukses(h.kirim(http.MethodPost, path+"/open", nil, lecturer.Access), http.StatusOK)
	manual := core02Manual(person, "present")
	manualHeaders := map[string]string{"Idempotency-Key": "scope-manual-core02"}
	h.harapSukses(core02Send(h, http.MethodPost, path+"/manual-checkins", manual, lecturer.Access, manualHeaders), http.StatusCreated)
	// The same key cannot be reused on another canonical mutation path.
	h.harapGalat(core02Send(h, http.MethodPost, path+"/manual-checkins", manual, lecturer.Access, createHeaders), http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	if response := h.kirim(http.MethodDelete, "/api/v1/classes/"+classID+"/dosen/"+lecturerID, nil, admin.Access); response.Status != http.StatusNoContent {
		t.Fatalf("remove assignment: %s", response.Rec.Body.String())
	}
	h.harapGalat(core02Send(h, http.MethodPost, "/api/v1/sessions", create, lecturer.Access, createHeaders), http.StatusNotFound, codeNotFound)
	h.harapGalat(core02Send(h, http.MethodPost, path+"/manual-checkins", manual, lecturer.Access, manualHeaders), http.StatusNotFound, codeNotFound)
}
