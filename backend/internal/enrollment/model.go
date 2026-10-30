// Package enrollment implements the explicitly injected, synthetic-only enrollment workflow.
package enrollment

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"time"
)

type Action string

const (
	Notice           Action = "notice"
	Grant            Action = "grant"
	Withdraw         Action = "withdraw"
	Status           Action = "status"
	Submit           Action = "submit"
	List             Action = "list"
	Detail           Action = "detail"
	Photo            Action = "photo"
	Approve          Action = "approve"
	Reject           Action = "reject"
	Retry            Action = "retry"
	AssistedWithdraw Action = "assisted_withdraw"
)

// Request contains server-resolved identity; no caller-supplied subject is accepted for self actions.
type Request struct {
	Action                                                  Action
	ActorID, SessionID, RequestID                           string
	IdempotencyKey, IfMatch, TargetID, ConsentID            string
	NoticeVersion, ReplacesConsentID                        string
	Accepted, Confirmed                                     bool
	Scope, RequestMethod, RequestRef                        string
	IdentityCheckMethod, AttestationVersion, CampusCheckRef string
	ReasonCode                                              string
	JPEG                                                    []byte
	FilterStatus, Cursor                                    string
	Limit                                                   int
	verifiedPhotoHash                                       string
	rateCharged                                             bool
}

type Result struct {
	HTTPStatus int
	Data       any
	ETag       string
	Photo      []byte
	Replay     bool
}

type Error struct {
	Code       string
	Status     int
	RetryAfter int
}

func (e *Error) Error() string { return e.Code }

const NoticeVersion = "synthetic-v1"
const NoticeText = "UJI SINTETIS SAJA. FaceLink memproses satu foto sintetis untuk pengujian alur persetujuan dan absensi satu kelas, ditinjau Admin dengan pemeriksaan kampus sintetis, lalu diproses lokal. Foto tertunda dienkripsi paling lama 72 jam dan dihapus saat aktif, ditolak, gagal, kedaluwarsa, atau persetujuan ditarik. Template sintetis terenkripsi dapat diakses hanya oleh layanan lokal berwenang. Tidak ada LLM/cloud, jaminan identitas, akurasi, atau liveness. Absensi manual tetap setara. Persetujuan dapat ditarik kapan saja melalui akun atau permintaan kampus kepada Admin; penghapusan fisik mempunyai batas SSD, paging, dan salinan eksternal. Tidak diizinkan untuk manusia atau produksi. Hubungi administrator pengujian kampus untuk pertanyaan atau penarikan."

type NoticeView struct {
	Version string `json:"version"`
	Digest  string `json:"digest"`
	Text    string `json:"text"`
}
type ConsentView struct {
	State       string     `json:"state"`
	RecordID    string     `json:"record_id,omitempty"`
	Notice      NoticeView `json:"notice"`
	GrantedAt   string     `json:"granted_at,omitempty"`
	WithdrawnAt string     `json:"withdrawn_at,omitempty"`
	Revision    int64      `json:"revision"`
}
type ClassRef struct {
	ID    string `json:"id"`
	Code  string `json:"code"`
	Title string `json:"title"`
}
type IdentityBindingView struct {
	AccountUserID      string     `json:"account_user_id"`
	PersonID           string     `json:"person_id"`
	ExternalCode       string     `json:"external_code"`
	AccountStatus      string     `json:"account_status"`
	AccountRole        string     `json:"account_role"`
	PersonStatus       string     `json:"person_status"`
	IdentityReference  string     `json:"identity_reference"`
	IdentityVerifiedAt string     `json:"identity_verified_at"`
	Eligible           bool       `json:"eligible"`
	Classes            []ClassRef `json:"classes"`
}
type StudentView struct {
	UserID       string     `json:"user_id"`
	PersonID     string     `json:"person_id"`
	DisplayName  string     `json:"display_name"`
	ExternalCode string     `json:"external_code"`
	Classes      []ClassRef `json:"classes"`
}
type ReviewView struct {
	ReviewerAdminID     string `json:"reviewer_admin_id"`
	Decision            string `json:"decision"`
	At                  string `json:"at"`
	IdentityCheckMethod string `json:"identity_check_method,omitempty"`
	CampusCheckRef      string `json:"campus_check_ref,omitempty"`
}
type HistoryView struct {
	AuditID    string `json:"audit_id"`
	Action     string `json:"action"`
	ActorID    string `json:"actor_id,omitempty"`
	Time       string `json:"time"`
	ReasonCode string `json:"reason_code,omitempty"`
}
type EnrollmentView struct {
	ServerNow          string               `json:"server_now"`
	Consent            *ConsentView         `json:"consent,omitempty"`
	IdentityBinding    *IdentityBindingView `json:"identity_binding,omitempty"`
	ArtifactIntegrity  string               `json:"artifact_integrity,omitempty"`
	EffectiveStatus    string               `json:"effective_status"`
	SubmissionStatus   string               `json:"submission_status,omitempty"`
	SubmissionID       string               `json:"submission_id,omitempty"`
	ConsentState       string               `json:"consent_state"`
	IdentityState      string               `json:"identity_state"`
	TemplateReady      bool                 `json:"template_ready"`
	SubmittedAt        string               `json:"submitted_at,omitempty"`
	ReviewedAt         string               `json:"reviewed_at,omitempty"`
	ProcessingDeadline string               `json:"processing_deadline,omitempty"`
	ReasonCode         string               `json:"reason_code,omitempty"`
	DeletionState      string               `json:"deletion_state"`
	Revision           int64                `json:"revision"`
	AllowedActions     []string             `json:"allowed_actions"`
	Student            *StudentView         `json:"student,omitempty"`
	PhotoAvailable     bool                 `json:"photo_available"`
	PhotoExpiresAt     string               `json:"photo_expires_at,omitempty"`
	Review             *ReviewView          `json:"review,omitempty"`
	History            []HistoryView        `json:"history,omitempty"`
	SyntheticOnly      bool                 `json:"synthetic_only"`
}
type MutationReceipt struct {
	OperationID     string `json:"operation_id"`
	ResourceID      string `json:"resource_id"`
	AppliedRevision int64  `json:"applied_revision"`
	Current         any    `json:"current"`
}
type ListItem struct {
	ID             string      `json:"id"`
	Student        StudentView `json:"student"`
	Status         string      `json:"status"`
	SubmittedAt    string      `json:"submitted_at"`
	PhotoExpiresAt string      `json:"photo_expires_at"`
}
type Page struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit"`
}
type ListView struct {
	ServerNow string     `json:"server_now"`
	Items     []ListItem `json:"items"`
	Page      Page       `json:"page"`
	Summary   struct {
		PendingCount int `json:"pending_count"`
	} `json:"summary"`
}

// Processor must be supplied explicitly. There is no built-in processor or production adapter.
// Process must honor cancellation; the service additionally fences late output and permits one in-flight call.
type Processor interface {
	Manifest() Manifest
	Process(context.Context, []byte) (ProcessingResult, error)
}
type Manifest struct {
	SyntheticOnly                                                   bool
	ModelID, ModelVersion, ModelChecksum                            string
	DetectorID, DetectorVersion, DetectorChecksum, PipelineRevision string
}

// Fingerprint identifies the complete stable model and preprocessing pipeline.
// Checksums are case-insensitive hexadecimal; all other identifiers are exact.
func (m Manifest) Fingerprint() string {
	h := sha256.New()
	synthetic := "false"
	if m.SyntheticOnly {
		synthetic = "true"
	}
	for _, value := range []string{
		"FaceLink:model-pipeline:1", synthetic, m.ModelID, m.ModelVersion,
		strings.ToLower(m.ModelChecksum), m.DetectorID, m.DetectorVersion,
		strings.ToLower(m.DetectorChecksum), m.PipelineRevision,
	} {
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(value)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(value))
	}
	return hex.EncodeToString(h.Sum(nil))
}

type ProcessingResult struct {
	Embedding []float32
	FaceCount int
	Manifest  Manifest
}

// ProcessingError is an allowlisted technical failure, never an identity decision.
type ProcessingError struct{ Code string }

func (e *ProcessingError) Error() string { return e.Code }

const PhotoTTL = 72 * time.Hour
