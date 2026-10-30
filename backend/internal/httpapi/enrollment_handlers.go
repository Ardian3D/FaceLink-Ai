package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type enrollmentHTTPSlots struct {
	preview sync.Map
	uploads chan struct{}
}

// No production switch: enrollment routes exist only on an explicitly injected service.
func (s Server) registerEnrollmentRoutes(r chi.Router) {
	if s.Enrollment == nil {
		return
	}
	slots := &enrollmentHTTPSlots{uploads: make(chan struct{}, 2)}
	noStore := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			next.ServeHTTP(w, r)
		})
	}
	r.Group(func(r chi.Router) {
		r.Use(noStore)
		r.Use(requireRole(s, auth.RoleMahasiswa))
		r.Get("/me/consent", s.enrollmentHandler(enrollment.Notice, slots))
		r.Post("/me/consent", s.enrollmentHandler(enrollment.Grant, slots))
		r.Post("/me/consent/withdraw", s.enrollmentHandler(enrollment.Withdraw, slots))
		r.Get("/me/face-enrollment", s.enrollmentHandler(enrollment.Status, slots))
		r.Post("/me/face-enrollment-submissions", s.enrollmentHandler(enrollment.Submit, slots))
	})
	r.Group(func(r chi.Router) {
		r.Use(noStore)
		r.Use(DenganPeranAdmin(s))
		r.Get("/face-enrollment-submissions", s.enrollmentHandler(enrollment.List, slots))
		r.Get("/face-enrollment-submissions/{submission_id}", s.enrollmentHandler(enrollment.Detail, slots))
		r.Get("/face-enrollment-submissions/{submission_id}/photo", s.enrollmentHandler(enrollment.Photo, slots))
		r.Post("/face-enrollment-submissions/{submission_id}/approve", s.enrollmentHandler(enrollment.Approve, slots))
		r.Post("/face-enrollment-submissions/{submission_id}/reject", s.enrollmentHandler(enrollment.Reject, slots))
		r.Post("/face-enrollment-submissions/{submission_id}/retry-processing", s.enrollmentHandler(enrollment.Retry, slots))
		r.Post("/people/{person_id}/consent/withdraw", s.enrollmentHandler(enrollment.AssistedWithdraw, slots))
	})
}

func enrollmentUUID(v string) bool {
	id, err := uuid.Parse(v)
	return err == nil && id.Version() == 4 && id.Variant() == uuid.RFC4122 && id.String() == v
}

func enrollmentHeader(r *http.Request, name string) (string, bool) {
	values := r.Header.Values(name)
	return r.Header.Get(name), len(values) == 1 && values[0] != ""
}

func (s Server) enrollmentHandler(action enrollment.Action, slots *enrollmentHTTPSlots) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		bad := func(status int, code string) {
			writeErrorCode(w, r, status, code, "Permintaan enrollment sintetis tidak memenuhi kontrak atau keadaan yang diizinkan.")
		}
		// Obtain the authenticated session ID using the unchanged session machinery;
		// the domain checks its current row again inside each transaction.
		principal, sess, err := s.authenticate(r.Context(), r)
		if err != nil {
			writeAuthFailure(w, r, err)
			return
		}
		req := enrollment.Request{Action: action, ActorID: principal.UserID, SessionID: sess.ID, RequestID: middleware.GetReqID(r.Context())}
		req.TargetID = chi.URLParam(r, "submission_id")
		if action == enrollment.AssistedWithdraw {
			req.TargetID = chi.URLParam(r, "person_id")
		}
		if req.TargetID != "" && !enrollmentUUID(req.TargetID) {
			bad(400, "VALIDATION_FAILED")
			return
		}
		query, parseErr := url.ParseQuery(r.URL.RawQuery)
		if parseErr != nil {
			bad(400, "VALIDATION_FAILED")
			return
		}
		if action == enrollment.List {
			if !core02Query(w, r, "status", "cursor", "limit") {
				return
			}
			req.FilterStatus, req.Cursor = query.Get("status"), query.Get("cursor")
			req.Limit = 20
			if value := query.Get("limit"); value != "" {
				n, e := strconv.Atoi(value)
				if e != nil || n < 1 || n > 50 {
					bad(400, "VALIDATION_FAILED")
					return
				}
				req.Limit = n
			}
		} else if !core02Query(w, r) {
			return
		}
		if action == enrollment.Photo {
			if r.Header.Get("Range") != "" || r.Header.Get("If-Range") != "" {
				bad(400, "VALIDATION_FAILED")
				return
			}
			if _, loaded := slots.preview.LoadOrStore(principal.UserID, true); loaded {
				w.Header().Set("Retry-After", "1")
				bad(429, "RATE_LIMITED")
				return
			}
			defer slots.preview.Delete(principal.UserID)
		}
		if r.Method == http.MethodPost {
			var ok bool
			req.IdempotencyKey, ok = enrollmentHeader(r, "Idempotency-Key")
			if !ok || !enrollmentUUID(req.IdempotencyKey) {
				bad(400, "VALIDATION_FAILED")
				return
			}
			if action == enrollment.Grant || action == enrollment.Approve || action == enrollment.Reject || action == enrollment.Retry {
				req.IfMatch, ok = enrollmentHeader(r, "If-Match")
				if !ok {
					bad(428, "PRECONDITION_REQUIRED")
					return
				}
				if len(req.IfMatch) < 3 || len(req.IfMatch) > 21 || req.IfMatch[0] != '"' || req.IfMatch[len(req.IfMatch)-1] != '"' {
					bad(400, "VALIDATION_FAILED")
					return
				}
				revision, revisionErr := strconv.ParseInt(req.IfMatch[1:len(req.IfMatch)-1], 10, 64)
				if revisionErr != nil || revision < 0 || strconv.FormatInt(revision, 10) != req.IfMatch[1:len(req.IfMatch)-1] {
					bad(400, "VALIDATION_FAILED")
					return
				}
			}
			media, params, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if e != nil {
				bad(415, "UNSUPPORTED_MEDIA_TYPE")
				return
			}
			if action == enrollment.Submit {
				if media != "image/jpeg" || len(params) != 0 {
					bad(415, "UNSUPPORTED_MEDIA_TYPE")
					return
				}
				req.ConsentID, ok = enrollmentHeader(r, "X-Consent-Id")
				if !ok || !enrollmentUUID(req.ConsentID) {
					bad(400, "VALIDATION_FAILED")
					return
				}
				if r.ContentLength < 1 {
					bad(400, "VALIDATION_FAILED")
					return
				}
				if r.ContentLength > enrollmentsecure.MaxImageBytes {
					bad(413, "IMAGE_TOO_LARGE")
					return
				}
				select {
				case slots.uploads <- struct{}{}:
					defer func() { <-slots.uploads }()
				default:
					w.Header().Set("Retry-After", "1")
					bad(429, "RATE_LIMITED")
					return
				}
				req.JPEG, err = io.ReadAll(io.LimitReader(r.Body, enrollmentsecure.MaxImageBytes+1))
				defer clear(req.JPEG)
				if err != nil {
					var tooLarge *http.MaxBytesError
					if errors.As(err, &tooLarge) {
						bad(413, "IMAGE_TOO_LARGE")
					} else {
						bad(400, "VALIDATION_FAILED")
					}
					return
				}
				if len(req.JPEG) > enrollmentsecure.MaxImageBytes {
					bad(413, "IMAGE_TOO_LARGE")
					return
				}
				if int64(len(req.JPEG)) != r.ContentLength {
					bad(400, "VALIDATION_FAILED")
					return
				}
			} else {
				if media != "application/json" || (len(params) > 0 && (len(params) != 1 || params["charset"] != "utf-8")) {
					bad(415, "UNSUPPORTED_MEDIA_TYPE")
					return
				}
				if r.ContentLength > 16<<10 {
					bad(413, "PAYLOAD_TOO_LARGE")
					return
				}
				raw, readErr := io.ReadAll(io.LimitReader(r.Body, (16<<10)+1))
				defer clear(raw)
				if readErr != nil || len(raw) > 16<<10 {
					bad(413, "PAYLOAD_TOO_LARGE")
					return
				}
				if !enrollmentDecode(req.Action, raw, &req) {
					bad(400, "VALIDATION_FAILED")
					return
				}
			}
		} else if r.ContentLength > 0 || len(r.TransferEncoding) > 0 {
			bad(400, "VALIDATION_FAILED")
			return
		}
		result, err := s.Enrollment.Execute(r.Context(), req)
		if err != nil {
			var domain *enrollment.Error
			if errors.As(err, &domain) {
				if domain.RetryAfter > 0 {
					w.Header().Set("Retry-After", strconv.Itoa(domain.RetryAfter))
				}
				bad(domain.Status, domain.Code)
			} else {
				bad(503, "BIOMETRIC_UNAVAILABLE")
			}
			return
		}
		if result.ETag != "" {
			w.Header().Set("ETag", result.ETag)
		}
		if action == enrollment.Photo {
			defer clear(result.Photo)
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Content-Length", strconv.Itoa(len(result.Photo)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(result.Photo)
			return
		}
		if result.HTTPStatus == http.StatusCreated && action == enrollment.Submit {
			w.Header().Set("Location", "/api/v1/me/face-enrollment")
		}
		if list, ok := result.Data.(enrollment.ListView); ok {
			writeJSON(w, result.HTTPStatus, map[string]any{"data": list.Items, "page": list.Page, "summary": list.Summary, "server_now": list.ServerNow, "request_id": middleware.GetReqID(r.Context())})
			return
		}
		writeData(w, r, result.HTTPStatus, result.Data)
	}
}

// Decode exactly one object, rejecting duplicates and action-inapplicable fields.
// Identity fields are not part of any self-service DTO.
func enrollmentDecode(action enrollment.Action, raw []byte, req *enrollment.Request) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return false
	}
	fields := map[string]json.RawMessage{}
	for dec.More() {
		k, e := dec.Token()
		if e != nil {
			return false
		}
		key, ok := k.(string)
		if !ok {
			return false
		}
		if _, exists := fields[key]; exists {
			return false
		}
		var value json.RawMessage
		if dec.Decode(&value) != nil {
			return false
		}
		fields[key] = value
	}
	if _, e := dec.Token(); e != nil {
		return false
	}
	if _, e := dec.Token(); e != io.EOF {
		return false
	}
	allowed := map[enrollment.Action][]string{
		enrollment.Grant:            {"notice_version", "accepted", "replaces_consent_id"},
		enrollment.Withdraw:         {"scope", "confirmed"},
		enrollment.AssistedWithdraw: {"scope", "confirmed", "request_method", "request_ref"},
		enrollment.Approve:          {"identity_check_method", "attestation_version", "confirmed", "campus_check_ref"},
		enrollment.Reject:           {"reason_code"}, enrollment.Retry: {"confirmed"},
	}[action]
	for k, v := range fields {
		found := false
		for _, a := range allowed {
			if k == a {
				found = true
				break
			}
		}
		if !found || string(v) == "null" {
			return false
		}
	}
	read := func(k string, dst any) bool { v, ok := fields[k]; return !ok || json.Unmarshal(v, dst) == nil }
	return read("notice_version", &req.NoticeVersion) && read("accepted", &req.Accepted) && read("replaces_consent_id", &req.ReplacesConsentID) && read("scope", &req.Scope) && read("confirmed", &req.Confirmed) && read("request_method", &req.RequestMethod) && read("request_ref", &req.RequestRef) && read("identity_check_method", &req.IdentityCheckMethod) && read("attestation_version", &req.AttestationVersion) && read("campus_check_ref", &req.CampusCheckRef) && read("reason_code", &req.ReasonCode)
}
