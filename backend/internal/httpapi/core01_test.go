package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func corePerson(h *harnessAuth, token, code string) string {
	h.t.Helper()
	data := h.harapSukses(h.kirim(http.MethodPost, "/api/v1/people", map[string]any{"external_code": code, "display_name": "Mahasiswa Sintetis " + code, "identity_reference": "roster-sintetis/" + code}, token), http.StatusCreated)
	return data["id"].(string)
}

func coreClass(h *harnessAuth, token, code string) string {
	h.t.Helper()
	return h.harapSukses(h.kirim(http.MethodPost, "/api/v1/classes", map[string]any{"code": code, "title": "Kelas Sintetis " + code}, token), http.StatusCreated)["id"].(string)
}

func TestCORE01AuthorizationOwnership(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("admin.core@kampus.example")
	lecturerID, lecturer := h.undangDanMasuk(admin.Access, "dosen.core@kampus.example", "Dosen Sintetis", "dosen")
	studentID, student := h.undangDanMasuk(admin.Access, "student.core@kampus.example", "Mahasiswa Sintetis", "mahasiswa")
	otherID, other := h.undangDanMasuk(admin.Access, "other.core@kampus.example", "Mahasiswa Lain Sintetis", "mahasiswa")
	p := corePerson(h, admin.Access, "SYN-001")
	q := corePerson(h, admin.Access, "SYN-002")
	c := coreClass(h, admin.Access, "SYN-A")
	d := coreClass(h, admin.Access, "SYN-B")
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+c+"/dosen/"+lecturerID, nil, admin.Access), http.StatusCreated)
	for _, pair := range [][2]string{{c, p}, {d, q}} {
		h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+pair[0]+"/members/"+pair[1], nil, admin.Access), http.StatusCreated)
	}
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/people/"+p+"/account-link", map[string]any{"user_id": studentID}, admin.Access), http.StatusOK)
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/people/"+q+"/account-link", map[string]any{"user_id": otherID}, admin.Access), http.StatusOK)
	for _, token := range []string{lecturer.Access, student.Access} {
		for _, request := range []struct {
			method, path string
			body         any
		}{
			{http.MethodPost, "/api/v1/people", map[string]any{"external_code": "FORGED", "display_name": "Forged"}},
			{http.MethodPatch, "/api/v1/people/" + p, map[string]any{"account_user_id": otherID}},
			{http.MethodDelete, "/api/v1/people/" + p, nil},
			{http.MethodPut, "/api/v1/people/" + p + "/account-link", map[string]any{"user_id": otherID}},
			{http.MethodDelete, "/api/v1/people/" + p + "/account-link", nil},
			{http.MethodPost, "/api/v1/classes", map[string]any{"code": "FORGED", "title": "Forged"}},
			{http.MethodPut, "/api/v1/classes/" + d + "/members/" + p, nil},
			{http.MethodDelete, "/api/v1/classes/" + c + "/members/" + p, nil},
		} {
			h.harapGalat(h.kirim(request.method, request.path, request.body, token), http.StatusForbidden, codeForbiddenRole)
		}
	}
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/people/"+q, nil, student.Access), http.StatusForbidden, codeForbiddenRole)
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/people", nil, student.Access), http.StatusForbidden, codeForbiddenRole)
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/people/"+q, nil, lecturer.Access), http.StatusNotFound, codeNotFound)
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/people?class_id="+d, nil, lecturer.Access), http.StatusNotFound, codeNotFound)
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/classes/"+d+"/members", nil, lecturer.Access), http.StatusNotFound, codeNotFound)
	people := h.harapSuksesLarik(h.kirim(http.MethodGet, "/api/v1/people", nil, lecturer.Access), http.StatusOK)
	if len(people) != 1 || !cariNilaiLarik(people, "id", p) {
		t.Fatalf("scope people: %v", people)
	}
	members := h.harapSuksesLarik(h.kirim(http.MethodGet, "/api/v1/classes/"+c+"/members", nil, lecturer.Access), http.StatusOK)
	if len(members) != 1 || !cariNilaiLarik(members, "person_id", p) {
		t.Fatalf("scope members: %v", members)
	}
	own := h.harapSuksesLarik(h.kirim(http.MethodGet, "/api/v1/me/classes?person_id="+q+"&user_id="+otherID, nil, student.Access), http.StatusOK)
	if len(own) != 1 || !cariNilaiLarik(own, "id", c) {
		t.Fatalf("client changed ownership: %v", own)
	}
	otherClasses := h.harapSuksesLarik(h.kirim(http.MethodGet, "/api/v1/me/classes", nil, other.Access), http.StatusOK)
	if len(otherClasses) != 1 || !cariNilaiLarik(otherClasses, "id", d) {
		t.Fatalf("other ownership: %v", otherClasses)
	}
	h.execSQL("UPDATE people SET identity_verified_at = NULL WHERE id = ?", q)
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/me/classes", nil, other.Access), http.StatusConflict, codeAccountNotLinked)
	if got := h.teksKolom("SELECT account_user_id FROM people WHERE id = ?", p); got != studentID {
		t.Fatalf("binding changed: %s", got)
	}
	if n := h.hitung("SELECT COUNT(*) FROM audit_events WHERE action='authz.denied'"); n != 18 {
		t.Fatalf("denial audit=%d want18", n)
	}
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/people", nil, ""), http.StatusUnauthorized, codeUnauthenticated)
}

func TestCORE01ValidationBindingAndIdempotency(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("admin.validation@kampus.example")
	studentID, student := h.undangDanMasuk(admin.Access, "student.validation@kampus.example", "Mahasiswa Sintetis", "mahasiswa")
	otherID, _ := h.undangDanMasuk(admin.Access, "other.validation@kampus.example", "Mahasiswa Lain", "mahasiswa")
	p := corePerson(h, admin.Access, "SYN-VALID")
	q := corePerson(h, admin.Access, "SYN-OTHER")
	path := "/api/v1/people/" + p + "/account-link"
	h.harapSukses(h.kirim(http.MethodPut, path, map[string]any{"user_id": studentID}, admin.Access), http.StatusOK)
	h.harapSukses(h.kirim(http.MethodPut, path, map[string]any{"user_id": studentID}, admin.Access), http.StatusOK)
	h.harapGalat(h.kirim(http.MethodPut, path, map[string]any{"user_id": otherID}, admin.Access), http.StatusConflict, codeConflict)
	h.harapGalat(h.kirim(http.MethodPut, "/api/v1/people/"+q+"/account-link", map[string]any{"user_id": studentID}, admin.Access), http.StatusConflict, codeConflict)
	if n := h.hitung("SELECT COUNT(*) FROM audit_events WHERE action='person.account_linked'"); n != 1 {
		t.Fatalf("link audit=%d", n)
	}
	h.harapGalat(h.kirim(http.MethodPost, "/api/v1/people", map[string]any{"external_code": "SYN-VALID", "display_name": "Duplicate"}, admin.Access), http.StatusConflict, "DUPLICATE_EXTERNAL_CODE")
	for _, body := range []map[string]any{{"account_user_id": otherID}, {"external_code": "REPLACED"}, {"identity_verified_at": "2026-01-01T00:00:00Z"}, {"display_name": "bad\nname"}} {
		h.harapGalat(h.kirim(http.MethodPatch, "/api/v1/people/"+p, body, admin.Access), http.StatusBadRequest, codeValidationFailed)
	}
	unverified := h.harapSukses(h.kirim(http.MethodPost, "/api/v1/people", map[string]any{"external_code": "UNVERIFIED", "display_name": "Unverified"}, admin.Access), http.StatusCreated)["id"].(string)
	h.harapGalat(h.kirim(http.MethodPut, "/api/v1/people/"+unverified+"/account-link", map[string]any{"user_id": otherID}, admin.Access), http.StatusConflict, codeConflict)
	c := coreClass(h, admin.Access, "SYN-VALID")
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+c+"/members/"+p, nil, admin.Access), http.StatusCreated)
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+c+"/members/"+p, nil, admin.Access), http.StatusOK)
	if n := h.hitung("SELECT COUNT(*) FROM audit_events WHERE action='class.member.added'"); n != 1 {
		t.Fatalf("member audit=%d", n)
	}
	h.harapSukses(h.kirim(http.MethodPatch, "/api/v1/people/"+p, map[string]any{"status": "inactive"}, admin.Access), http.StatusOK)
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/me/classes", nil, student.Access), http.StatusConflict, codeAccountNotLinked)
	h.harapSukses(h.kirim(http.MethodPatch, "/api/v1/people/"+p, map[string]any{"status": "active"}, admin.Access), http.StatusOK)
	classes := h.harapSuksesLarik(h.kirim(http.MethodGet, "/api/v1/me/classes", nil, student.Access), http.StatusOK)
	if len(classes) != 1 || !cariNilaiLarik(classes, "id", c) {
		t.Fatalf("reactivated ownership: %v", classes)
	}
	if r := h.kirim(http.MethodDelete, path, nil, admin.Access); r.Status != http.StatusNoContent {
		t.Fatalf("unlink: %s", r.Rec.Body.String())
	}
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/me/classes", nil, student.Access), http.StatusConflict, codeAccountNotLinked)
	h.harapSukses(h.kirim(http.MethodPatch, "/api/v1/people/"+q, map[string]any{"status": "inactive"}, admin.Access), http.StatusOK)
	h.harapGalat(h.kirim(http.MethodPut, "/api/v1/people/"+q+"/account-link", map[string]any{"user_id": otherID}, admin.Access), http.StatusConflict, codeConflict)
	for _, metadata := range h.metadataAudit() {
		if containsCorePrivate(metadata) {
			t.Fatalf("identity leaked in audit: %s", metadata)
		}
	}
}

func containsCorePrivate(s string) bool {
	for _, value := range []string{"SYN-VALID", "roster-sintetis", "Mahasiswa Sintetis"} {
		if strings.Contains(s, value) {
			return true
		}
	}
	return false
}

func TestCORE01PeoplePagination(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("admin.pages@kampus.example")
	a := corePerson(h, admin.Access, "PAGE-A")
	b := corePerson(h, admin.Access, "PAGE-B")
	first := h.kirim(http.MethodGet, "/api/v1/people?limit=1", nil, admin.Access)
	data := h.harapSuksesLarik(first, http.StatusOK)
	page := first.Body["page"].(map[string]any)
	if len(data) != 1 || page["has_more"] != true {
		t.Fatalf("first page: %v", first.Body)
	}
	next := h.kirim(http.MethodGet, "/api/v1/people?limit=1&cursor="+page["next_cursor"].(string), nil, admin.Access)
	second := h.harapSuksesLarik(next, http.StatusOK)
	if len(second) != 1 || data[0].(map[string]any)["id"] == second[0].(map[string]any)["id"] {
		t.Fatalf("duplicate pagination: %v %v", data, second)
	}
	for _, id := range []string{a, b} {
		if !cariNilaiLarik(append(data, second...), "id", id) {
			t.Fatalf("missing %s", id)
		}
	}
	c := coreClass(h, admin.Access, "PAGE-A")
	d := coreClass(h, admin.Access, "PAGE-B")
	classes := h.kirim(http.MethodGet, "/api/v1/classes?limit=1", nil, admin.Access)
	h.harapSuksesLarik(classes, http.StatusOK)
	cursor := classes.Body["page"].(map[string]any)["next_cursor"].(string)
	h.execSQL("UPDATE classes SET deleted_at = created_at WHERE id = ?", c)
	remaining := h.harapSuksesLarik(h.kirim(http.MethodGet, "/api/v1/classes?limit=1&cursor="+cursor, nil, admin.Access), http.StatusOK)
	if len(remaining) != 1 || !cariNilaiLarik(remaining, "id", d) {
		t.Fatalf("deleted cursor row replayed page: %v", remaining)
	}
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/people?cursor="+cursor, nil, admin.Access), http.StatusBadRequest, codeValidationFailed)
}
