package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/go-chi/chi/v5"
)

const (
	core02SessionCursor    = "attendance-session"
	core02AttendanceCursor = "attendance-person"
	core02OwnCursor        = "own-attendance"
)

// registerCore02Routes must run inside the authenticated router group.
func (s Server) registerCore02Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(DenganPeranAdminDosen(s))
		r.Get("/sessions", s.handleListClassSessions)
		r.Post("/sessions", s.handleCreateClassSession)
		r.Get("/sessions/{session_id}", s.handleGetClassSession)
		r.Post("/sessions/{session_id}/open", s.handleOpenClassSession)
		r.Post("/sessions/{session_id}/close", s.handleCloseClassSession)
		r.Get("/sessions/{session_id}/manual-checkins", s.handleListManualAttendance)
		r.Post("/sessions/{session_id}/manual-checkins", s.handleRecordManualAttendance)
		r.Get("/sessions/{session_id}/attendance", s.handleListSessionAttendance)
	})
	r.With(requireRole(s, auth.RoleMahasiswa)).Get("/me/attendance", s.handleOwnAttendance)
}

func core02BadRequest(w http.ResponseWriter, r *http.Request) {
	writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Parameter atau badan permintaan tidak memenuhi aturan.")
}

func core02Query(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	for name, values := range r.URL.Query() {
		found := false
		for _, key := range allowed {
			if name == key {
				found = true
				break
			}
		}
		if !found || len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			core02BadRequest(w, r)
			return false
		}
	}
	return true
}

func core02Times(w http.ResponseWriter, r *http.Request) (*time.Time, *time.Time, bool) {
	var dates [2]*time.Time
	for i, key := range []string{"from", "to"} {
		if value := r.URL.Query().Get(key); value != "" {
			parsed, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				core02BadRequest(w, r)
				return nil, nil, false
			}
			parsed = parsed.UTC()
			dates[i] = &parsed
		}
	}
	if dates[0] != nil && dates[1] != nil && dates[0].After(*dates[1]) {
		core02BadRequest(w, r)
		return nil, nil, false
	}
	return dates[0], dates[1], true
}

func core02Page(w http.ResponseWriter, r *http.Request, kind string) (permintaanHalaman, bool) {
	page, ok := bacaHalaman(w, r, kind)
	if !ok {
		return page, false
	}
	if page.Lanjut {
		if kind == core02AttendanceCursor {
			if page.AfterCreatedAt != "" {
				core02BadRequest(w, r)
				return page, false
			}
		} else if _, err := time.Parse(time.RFC3339Nano, page.AfterCreatedAt); err != nil {
			core02BadRequest(w, r)
			return page, false
		}
	}
	return page, true
}

func core02Key(w http.ResponseWriter, r *http.Request) (string, bool) {
	values := r.Header.Values("Idempotency-Key")
	if len(values) == 0 {
		return "", true
	}
	if len(values) != 1 || len(values[0]) < 8 || len(values[0]) > 128 {
		core02BadRequest(w, r)
		return "", false
	}
	for _, c := range []byte(values[0]) {
		if c < 32 || c > 126 {
			core02BadRequest(w, r)
			return "", false
		}
	}
	return values[0], true
}

func (s Server) core02Error(w http.ResponseWriter, r *http.Request, err error) {
	status, code := http.StatusConflict, ""
	switch {
	case errors.Is(err, auth.ErrSessionAlreadyOpen):
		code = "SESSION_ALREADY_OPEN"
	case errors.Is(err, auth.ErrSessionClosed):
		code = "SESSION_CLOSED"
	case errors.Is(err, auth.ErrSessionNotOpen):
		status, code = http.StatusUnprocessableEntity, "SESSION_NOT_OPEN"
	case errors.Is(err, auth.ErrDeviceAlreadyBound):
		code = "DEVICE_ALREADY_BOUND"
	case errors.Is(err, auth.ErrDeviceUnavailable):
		status, code = http.StatusForbidden, "DEVICE_REVOKED"
	case errors.Is(err, auth.ErrClassMismatch):
		status, code = http.StatusUnprocessableEntity, "CLASS_MISMATCH"
	case errors.Is(err, auth.ErrReasonRequired):
		status, code = http.StatusBadRequest, "REASON_REQUIRED"
	case errors.Is(err, auth.ErrAlreadyCheckedIn):
		code = "ALREADY_CHECKED_IN"
	case errors.Is(err, auth.ErrIdempotencyKeyReused):
		code = "IDEMPOTENCY_KEY_REUSED"
	case errors.Is(err, auth.ErrAttendanceValidation):
		status, code = http.StatusUnprocessableEntity, codeValidationFailed
	case errors.Is(err, auth.ErrNotLinked):
		code = codeAccountNotLinked
	default:
		s.tulisGalatDomain(w, r, err)
		return
	}
	writeErrorCode(w, r, status, code, "Operasi tidak memenuhi keadaan atau kebijakan absensi.")
}

func (s Server) handleCreateClassSession(w http.ResponseWriter, r *http.Request) {
	var input auth.CreateClassSessionInput
	if err := decodeJSON(w, r, &input); err != nil || input.ClassID == "" || input.ClosesAt.IsZero() {
		core02BadRequest(w, r)
		return
	}
	key, ok := core02Key(w, r)
	if !ok {
		return
	}
	result, err := s.Accounts.CreateClassSession(r.Context(), principalID(r), input, key)
	if err != nil {
		s.core02Error(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, result)
}

func (s Server) handleListClassSessions(w http.ResponseWriter, r *http.Request) {
	if !core02Query(w, r, "class_id", "status", "from", "to", "cursor", "limit") {
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "draft" && status != "open" && status != "closed" {
		core02BadRequest(w, r)
		return
	}
	from, to, ok := core02Times(w, r)
	if !ok {
		return
	}
	page, ok := core02Page(w, r, core02SessionCursor)
	if !ok {
		return
	}
	rows, err := s.Accounts.ListClassSessions(r.Context(), principalID(r), auth.ClassSessionQuery{ClassID: r.URL.Query().Get("class_id"), Status: status, From: from, To: to, AfterCreatedAt: page.AfterCreatedAt, AfterID: page.AfterID, Limit: page.Limit + 1})
	if err != nil {
		s.core02Error(w, r, err)
		return
	}
	more := len(rows) > page.Limit
	next := ""
	if more {
		rows = rows[:page.Limit]
		last := rows[len(rows)-1]
		next = bikinKursor(core02SessionCursor, audit.FormatWaktu(last.CreatedAt), last.ID)
	}
	if rows == nil {
		rows = []auth.ClassSession{}
	}
	writePage(w, r, rows, next, more, page.Limit)
}

func (s Server) handleGetClassSession(w http.ResponseWriter, r *http.Request) {
	result, err := s.Accounts.GetClassSession(r.Context(), principalID(r), chi.URLParam(r, "session_id"))
	if err != nil {
		s.core02Error(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, result)
}

func (s Server) handleOpenClassSession(w http.ResponseWriter, r *http.Request) {
	var empty struct{}
	if err := decodeJSON(w, r, &empty); err != nil && !errors.Is(err, io.EOF) {
		core02BadRequest(w, r)
		return
	}
	result, err := s.Accounts.OpenClassSession(r.Context(), principalID(r), chi.URLParam(r, "session_id"))
	if err != nil {
		s.core02Error(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, result)
}

func (s Server) handleCloseClassSession(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(w, r, &input); err != nil && !errors.Is(err, io.EOF) {
		core02BadRequest(w, r)
		return
	}
	result, err := s.Accounts.CloseClassSession(r.Context(), principalID(r), chi.URLParam(r, "session_id"), input.Reason)
	if err != nil {
		s.core02Error(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, result)
}

func (s Server) handleRecordManualAttendance(w http.ResponseWriter, r *http.Request) {
	var input auth.ManualAttendanceInput
	if err := decodeJSON(w, r, &input); err != nil || input.PersonID == "" || (input.Status != "present" && input.Status != "late" && input.Status != "excused") {
		core02BadRequest(w, r)
		return
	}
	key, ok := core02Key(w, r)
	if !ok {
		return
	}
	result, status, err := s.Accounts.RecordManualAttendance(r.Context(), principalID(r), chi.URLParam(r, "session_id"), input, key)
	if err != nil {
		s.core02Error(w, r, err)
		return
	}
	writeData(w, r, status, result)
}

func (s Server) handleListManualAttendance(w http.ResponseWriter, r *http.Request) {
	if !core02Query(w, r) {
		return
	}
	rows, err := s.Accounts.ListManualAttendance(r.Context(), principalID(r), chi.URLParam(r, "session_id"))
	if err != nil {
		s.core02Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []auth.AttendanceRecord{}
	}
	writeData(w, r, http.StatusOK, rows)
}

type core02AttendanceEnvelope struct {
	Data      []auth.AttendanceRecord `json:"data"`
	Counts    *auth.AttendanceCounts  `json:"counts,omitempty"`
	Page      halamanPayload          `json:"page"`
	RequestID string                  `json:"request_id"`
}

func (s Server) handleListSessionAttendance(w http.ResponseWriter, r *http.Request) {
	if !core02Query(w, r, "include", "cursor", "limit") {
		return
	}
	include := r.URL.Query().Get("include")
	if include != "" && include != "counts" {
		core02BadRequest(w, r)
		return
	}
	page, ok := core02Page(w, r, core02AttendanceCursor)
	if !ok {
		return
	}
	// The store checks current actor and target scope before any cache decision.
	rows, counts, err := s.Accounts.ListSessionAttendance(r.Context(), principalID(r), chi.URLParam(r, "session_id"), auth.AttendanceQuery{AfterID: page.AfterID, Limit: page.Limit + 1})
	if err != nil {
		s.core02Error(w, r, err)
		return
	}
	more := len(rows) > page.Limit
	next := ""
	if more {
		rows = rows[:page.Limit]
		next = bikinKursor(core02AttendanceCursor, "", rows[len(rows)-1].PersonID)
	}
	if rows == nil {
		rows = []auth.AttendanceRecord{}
	}
	payload := core02AttendanceEnvelope{Data: rows, Page: halamanPayload{NextCursor: next, HasMore: more, Limit: page.Limit}}
	if include == "counts" {
		payload.Counts = &counts
	}
	// Request IDs are transport metadata, not part of the cached representation.
	encoded, err := json.Marshal(struct {
		Query    string
		Response core02AttendanceEnvelope
	}{r.URL.Query().Encode(), payload})
	if err != nil {
		s.core02Error(w, r, err)
		return
	}
	digest := sha256.Sum256(encoded)
	etag := `"` + hex.EncodeToString(digest[:]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("Vary", "Authorization")
	for _, candidate := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	payload.RequestID = reqID(r)
	writeJSON(w, http.StatusOK, payload)
}

func (s Server) handleOwnAttendance(w http.ResponseWriter, r *http.Request) {
	// A whitelist rejects even empty person_id/user_id selectors, rather than
	// silently accepting a client-supplied identity on the self-service route.
	if !core02Query(w, r, "class_id", "from", "to", "cursor", "limit") {
		return
	}
	from, to, ok := core02Times(w, r)
	if !ok {
		return
	}
	page, ok := core02Page(w, r, core02OwnCursor)
	if !ok {
		return
	}
	rows, err := s.Accounts.ListOwnAttendance(r.Context(), principalID(r), auth.OwnAttendanceQuery{ClassID: r.URL.Query().Get("class_id"), From: from, To: to, AfterCreatedAt: page.AfterCreatedAt, AfterID: page.AfterID, Limit: page.Limit + 1})
	if err != nil {
		s.core02Error(w, r, err)
		return
	}
	more := len(rows) > page.Limit
	next := ""
	if more {
		rows = rows[:page.Limit]
		last := rows[len(rows)-1]
		next = bikinKursor(core02OwnCursor, audit.FormatWaktu(last.CreatedAt), last.SessionID)
	}
	if rows == nil {
		rows = []auth.OwnAttendanceRecord{}
	}
	writePage(w, r, rows, next, more, page.Limit)
}
