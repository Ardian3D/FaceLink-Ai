package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
)

func TestCORE02ManualRemainsAvailableWithoutBiometrics(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("availability.admin@example.invalid")
	h.srv.Config.AIEngineURL = "http://127.0.0.1:1"
	h.router = NewRouter(h.srv)
	ready := h.kirim(http.MethodGet, "/api/v1/readyz", nil, "")
	if ready.Status != http.StatusOK {
		t.Fatalf("core readiness: %s", ready.Rec.Body.String())
	}
	manual := ready.Body["manual_attendance"].(map[string]any)
	biometric := ready.Body["biometrics"].(map[string]any)
	if manual["status"] != "ready" || biometric["status"] != "unavailable" || biometric["reason"] != "BIOMETRIC_NOT_IMPLEMENTED" {
		t.Fatalf("separate readiness: %v", ready.Body)
	}
	classID := coreClass(h, admin.Access, "AVAILABILITY")
	personID := corePerson(h, admin.Access, "AVAILABILITY-P")
	h.harapSukses(h.kirim(http.MethodPut, "/api/v1/classes/"+classID+"/members/"+personID, nil, admin.Access), http.StatusCreated)
	sess, err := h.srv.Accounts.CreateClassSession(context.Background(), admin.UserID, auth.CreateClassSessionInput{ClassID: classID, ClosesAt: h.clk.Now().Add(time.Hour)}, "")
	if err != nil {
		t.Fatal(err)
	}
	h.harapSukses(h.kirim(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/open", nil, admin.Access), http.StatusOK)
	h.harapSukses(h.kirim(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/manual-checkins", map[string]any{"person_id": personID, "status": "present", "reason": "Fallback: biometric subsystem unavailable"}, admin.Access), http.StatusCreated)
	if n := h.hitung("SELECT COUNT(*) FROM consent_records"); n != 0 {
		t.Fatalf("unexpected consent writes %d", n)
	}
	if n := h.hitung("SELECT COUNT(*) FROM attendance_records WHERE source <> 'manual'"); n != 0 {
		t.Fatalf("nonmanual attendance written %d", n)
	}
	if n := h.hitung("SELECT COUNT(*) FROM audit_events WHERE action = 'attendance.manual_corrected' AND actor_id = ?", admin.UserID); n != 1 {
		t.Fatalf("fallback mutation audit %d", n)
	}
	if n := h.hitung("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='face_templates'"); n != 0 {
		t.Fatal("biometric schema created")
	}
	// No recognition route is activated; arbitrary clients cannot fabricate success.
	denied := h.kirim(http.MethodPost, "/api/v1/devices/synthetic/recognition-attempts", nil, admin.Access)
	if denied.Status != http.StatusNotFound {
		t.Fatalf("recognition unexpectedly active: %d", denied.Status)
	}
	if n := h.hitung("SELECT COUNT(*) FROM attendance_records"); n != 1 {
		t.Fatalf("recognition request changed attendance: %d", n)
	}
}

func TestCORE02ReadinessDoesNotClaimManualWithoutAuth(t *testing.T) {
	h := newHarnessAuth(t)
	h.srv.Accounts = nil
	h.router = NewRouter(h.srv)
	ready := h.kirim(http.MethodGet, "/api/v1/readyz", nil, "")
	if ready.Status != http.StatusOK {
		t.Fatalf("infrastructure readiness: %s", ready.Rec.Body.String())
	}
	manual := ready.Body["manual_attendance"].(map[string]any)
	if manual["status"] != "unavailable" || manual["reason"] != "AUTH_OR_SCHEMA_UNAVAILABLE" {
		t.Fatalf("manual incorrectly ready: %v", manual)
	}
	denied := h.kirim(http.MethodPost, "/api/v1/sessions", map[string]any{}, "synthetic")
	if denied.Status != http.StatusServiceUnavailable {
		t.Fatalf("missing auth must fail closed: %d", denied.Status)
	}
}
