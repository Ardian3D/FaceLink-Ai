package httpapi

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/emailotp"
)

// OTP transport is verified at the actual connection, never a forwarded header.
func (s Server) emailReady(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if r.TLS == nil {
		writeErrorCode(w, r, http.StatusForbidden, "HTTPS_REQUIRED", "Autentikasi memerlukan HTTPS.")
		return false
	}
	if s.EmailOTP == nil || !s.authSiap() {
		writeErrorCode(w, r, http.StatusServiceUnavailable, codeAuthUnavailable, "Layanan autentikasi belum tersedia.")
		return false
	}
	return true
}

func remoteOTPIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return "unknown"
}

func (s Server) emailError(w http.ResponseWriter, r *http.Request, err error) {
	var rate *emailotp.RateError
	switch {
	case errors.As(err, &rate):
		w.Header().Set("Retry-After", itoa(rate.RetryAfter))
		s.catatPenolakan(r, "email_rate_limited")
		writeErrorCode(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Terlalu banyak percobaan. Silakan coba lagi nanti.")
	case errors.Is(err, emailotp.ErrValidation):
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Format email, challenge, atau kode tidak valid.")
	case errors.Is(err, emailotp.ErrInvalid):
		s.catatPenolakan(r, "email_otp_invalid")
		writeErrorCode(w, r, http.StatusUnauthorized, "OTP_INVALID", "Kode tidak dapat diverifikasi. Minta kode baru dan coba lagi.")
	default:
		s.catatPenolakan(r, "email_auth_unavailable")
		writeErrorCode(w, r, http.StatusServiceUnavailable, codeAuthUnavailable, "Layanan autentikasi belum tersedia.")
	}
}

func (s Server) handleEmailRequest(w http.ResponseWriter, r *http.Request) {
	if !s.emailReady(w, r) {
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		s.emailError(w, r, emailotp.ErrValidation)
		return
	}
	challenge, err := s.EmailOTP.Request(r.Context(), req.Email, remoteOTPIP(r))
	if err != nil {
		s.emailError(w, r, err)
		return
	}
	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorSystem, Action: "auth.email.requested",
		EntityType: "user", Result: audit.ResultSuccess,
		Metadata: map[string]string{"reason_code": "email_request_accepted"},
	})
	writeData(w, r, http.StatusAccepted, challenge)
}

func (s Server) handleEmailVerify(w http.ResponseWriter, r *http.Request) {
	if !s.emailReady(w, r) {
		return
	}
	var req struct {
		ChallengeID string          `json:"challenge_id"`
		Code        json.RawMessage `json:"code"`
	}
	decodeErr := decodeJSON(w, r, &req)
	var code string
	if decodeErr != nil || json.Unmarshal(req.Code, &code) != nil {
		code = ""
	}
	// Even malformed codes debit the persistent counters and attempt budget.
	user, err := s.EmailOTP.Verify(r.Context(), req.ChallengeID, code, remoteOTPIP(r))
	if err != nil {
		s.emailError(w, r, err)
		return
	}
	if decodeErr != nil {
		s.emailError(w, r, emailotp.ErrValidation)
		return
	}
	// Consume is already committed. Session failure must never revive the code.
	if user.Status != auth.StatusActive || !user.Role.Valid() {
		s.emailError(w, r, emailotp.ErrInvalid)
		return
	}
	pair, err := s.Sessions.Issue(r.Context(), user.ID, ringkasUserAgent(r.UserAgent()))
	if err != nil {
		s.emailError(w, r, emailotp.ErrUnavailable)
		return
	}
	s.catatAudit(r, audit.Event{
		ActorType: audit.ActorUser, ActorID: user.ID, Action: "auth.email.succeeded",
		EntityType: "session_token", EntityID: pair.SessionID, Result: audit.ResultSuccess,
		Metadata: map[string]string{"provider": "email", "reason_code": "email_verified"},
	})
	s.writeSesiResponse(w, r, user, pair)
}
