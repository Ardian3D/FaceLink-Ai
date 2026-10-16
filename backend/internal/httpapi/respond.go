package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// Bentuk respons mengikuti docs/API_CONTRACT.md §1.3 dan §1.4: sukses memakai
// {"data": ..., "request_id": ...} dan galat memakai
// {"error": {"code", "message", "request_id"}}.
//
// Catatan: endpoint kesehatan dan penanganan rute tidak dikenal pada FL-005
// masih memakai bentuk datar yang lebih tua. Perbedaan itu dicatat sebagai
// ketidakcocokan kontrak yang belum diseragamkan, bukan diubah diam-diam di
// tengah tugas autentikasi.

// batasBadanAuth membatasi ukuran badan permintaan autentikasi. ID token Google
// berukuran beberapa kilobita; batas satu mebibita dari bodyLimit tetap berlaku
// sebagai lapisan luar.
const batasBadanAuth = 64 << 10

// dataEnvelope adalah amplop respons sukses.
type dataEnvelope struct {
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

// galatEnvelope adalah amplop respons galat.
type galatEnvelope struct {
	Error galatIsi `json:"error"`
}

type galatIsi struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// writeData menulis respons sukses dalam amplop kontrak.
func writeData(w http.ResponseWriter, r *http.Request, status int, data any) {
	writeJSON(w, status, dataEnvelope{Data: data, RequestID: middleware.GetReqID(r.Context())})
}

// writeErrorCode menulis galat dalam amplop kontrak dengan kode stabil.
func writeErrorCode(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, galatEnvelope{Error: galatIsi{
		Code:      code,
		Message:   message,
		RequestID: middleware.GetReqID(r.Context()),
	}})
}

// errAuth adalah penolakan autentikasi/otorisasi beserta kode status kontrak.
type errAuth struct {
	status int
	code   string
}

func (e errAuth) Error() string { return e.code }

// writeAuthFailure menulis penolakan autentikasi memakai amplop kontrak.
func writeAuthFailure(w http.ResponseWriter, r *http.Request, err error) {
	var ea errAuth
	if !errors.As(err, &ea) {
		writeErrorCode(w, r, http.StatusUnauthorized, codeUnauthenticated, "Permintaan memerlukan autentikasi.")
		return
	}
	writeErrorCode(w, r, ea.status, ea.code, pesanGalatAuth(ea.code))
}

// pesanGalatAuth memetakan kode stabil ke pesan manusia. Pesan tidak pernah
// menjelaskan alasan detail pemeriksaan agar tidak membocorkan keberadaan akun.
func pesanGalatAuth(code string) string {
	switch code {
	case codeTokenExpired:
		return "Masa berlaku token akses sudah berakhir."
	case codeSessionRevoked:
		return "Sesi tidak berlaku lagi. Silakan masuk kembali."
	case codeForbiddenRole:
		return "Peran tidak memiliki akses ke operasi ini."
	case codeIdentityTokenInvalid:
		return "Bukti identitas tidak dapat diverifikasi."
	case codeIdentityNotLinked:
		return "Akun ini belum diberi akses ke aplikasi."
	case codeGoogleUnavailable:
		return "Layanan identitas sedang tidak dapat dihubungi."
	default:
		return "Permintaan memerlukan autentikasi."
	}
}

// decodeJSON membaca badan JSON dengan batas ukuran dan menolak bidang yang
// tidak dikenal agar kesalahan ketik tidak lewat tanpa disadari.
func decodeJSON(w http.ResponseWriter, r *http.Request, tujuan any) error {
	batas := http.MaxBytesReader(w, r.Body, batasBadanAuth)
	dec := json.NewDecoder(batas)
	dec.DisallowUnknownFields()
	if err := dec.Decode(tujuan); err != nil {
		return err
	}
	// Hanya satu objek JSON yang diterima.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("badan permintaan memuat lebih dari satu objek JSON")
	}
	return nil
}

// Kode galat stabil yang dipakai handler autentikasi dan administrasi.
const (
	codeUnauthenticated       = "UNAUTHENTICATED"
	codeTokenExpired          = "TOKEN_EXPIRED"
	codeSessionRevoked        = "SESSION_REVOKED"
	codeForbiddenRole         = "FORBIDDEN_ROLE"
	codeGoogleUnavailable     = "GOOGLE_UNAVAILABLE"
	codeIdentityTokenInvalid  = "IDENTITY_TOKEN_INVALID"
	codeIdentityNotLinked     = "IDENTITY_NOT_LINKED"
	codeAccountNotLinked      = "ACCOUNT_NOT_LINKED"
	codeLastAdminProtected    = "LAST_ADMIN_PROTECTED"
	codeSelfAccountProtected  = "SELF_ACCOUNT_PROTECTED"
	codeNotFound              = "NOT_FOUND"
	codeValidationFailed      = "VALIDATION_FAILED"
	codeMissingField          = "MISSING_FIELD"
	codeConflict              = "CONFLICT"
	codeInternalError         = "INTERNAL_ERROR"
	codeAuthUnavailable       = "AUTH_UNAVAILABLE"
	codeMethodNotAllowed      = "METHOD_NOT_ALLOWED"
	codeNoActiveSession       = "NO_ACTIVE_SESSION"
	codeInvalidNonce          = "INVALID_NONCE"
	codeRefreshTokenRequired  = "REFRESH_TOKEN_REQUIRED"
	codeIdentityTokenRequired = "ID_TOKEN_REQUIRED"
)
