package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

func TestDefaultStagingDoesNotSeedAdminSubmissionsOrControls(t *testing.T) {
	s, err := newStaging(context.Background(), filepath.Join("..", "..", "migrations"), false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	if s.control != nil || s.fixtures != nil {
		t.Fatal("default ENR02A runtime enabled opt-in fixtures")
	}
	var submissions int
	if err = s.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM face_enrollment_submissions`).Scan(&submissions); err != nil || submissions != 0 {
		t.Fatalf("default submissions=%d err=%v", submissions, err)
	}
	if err = s.applyControl(context.Background(), fixtureControl{Command: "worker.pause"}); err == nil {
		t.Fatal("default runtime accepted fixture control")
	}
}

func TestAdminFixturesUseRealReviewProcessingExpiryAndRoleGuards(t *testing.T) {
	ctx := context.Background()
	s, err := newStaging(ctx, filepath.Join("..", "..", "migrations"), true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	if !s.control.workerPaused() {
		t.Fatal("Admin runtime worker must start paused")
	}
	adminSession, err := s.fixtureSession(ctx, s.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	execute := func(r enrollment.Request) enrollment.Result {
		t.Helper()
		r.ActorID, r.SessionID, r.RequestID = s.admin.ID, adminSession, ids.New()
		v, err := s.service.Execute(ctx, r)
		if err != nil {
			t.Fatalf("%s: %v", r.Action, err)
		}
		return v
	}
	control := func(c fixtureControl) {
		t.Helper()
		if err := s.applyControl(ctx, c); err != nil {
			t.Fatalf("%s: %v", c.Command, err)
		}
	}
	queue := execute(enrollment.Request{Action: enrollment.List, FilterStatus: "PENDING_APPROVAL", Limit: 1}).Data.(enrollment.ListView)
	if len(queue.Items) != 1 || !queue.Page.HasMore || queue.Summary.PendingCount != 4 || queue.ServerNow == "" {
		t.Fatalf("fixture queue=%+v", queue)
	}
	for _, name := range []string{"pending-approve", "pending-reject", "pending-expiry", "pending-conflict", "failed-processing"} {
		fixture := s.fixtures[name]
		v := execute(enrollment.Request{Action: enrollment.Detail, TargetID: fixture.SubmissionID}).Data.(enrollment.EnrollmentView)
		if v.SubmissionStatus != fixture.Status || v.IdentityBinding.IdentityReference != fixture.IdentityReference || v.IdentityBinding.PersonID != fixture.PersonID || !v.SyntheticOnly || len(v.Student.Classes) != 1 {
			t.Fatalf("fixture %s=%+v", name, v)
		}
		if len(fixture.IdentityReference) != 36 || fixture.IdentityReference[14] != '4' {
			t.Fatal("synthetic campus reference must be a UUIDv4")
		}
	}
	failed := s.fixtures["failed-processing"]
	detail := execute(enrollment.Request{Action: enrollment.Detail, TargetID: failed.SubmissionID})
	v := detail.Data.(enrollment.EnrollmentView)
	if v.ReasonCode != "BIOMETRIC_UNAVAILABLE" || !v.PhotoAvailable || !slices.Contains(v.AllowedActions, "RETRY_PROCESSING") {
		t.Fatalf("retained retry fixture=%+v", v)
	}
	retry := execute(enrollment.Request{Action: enrollment.Retry, TargetID: failed.SubmissionID, IdempotencyKey: ids.New(), IfMatch: detail.ETag, Confirmed: true})
	if retry.HTTPStatus != 202 || retry.Data.(enrollment.MutationReceipt).Current.(enrollment.EnrollmentView).SubmissionStatus != "APPROVED_PROCESSING" {
		t.Fatal("retry did not return genuine processing receipt")
	}
	v = execute(enrollment.Request{Action: enrollment.Detail, TargetID: failed.SubmissionID}).Data.(enrollment.EnrollmentView)
	if v.TemplateReady || v.SubmissionStatus != "APPROVED_PROCESSING" {
		t.Fatal("paused worker did not preserve observable processing state")
	}
	control(fixtureControl{Command: "worker.step"})
	v = execute(enrollment.Request{Action: enrollment.Detail, TargetID: failed.SubmissionID}).Data.(enrollment.EnrollmentView)
	if !v.TemplateReady || v.SubmissionStatus != "ACTIVE" {
		t.Fatalf("real retry activation=%+v", v)
	}
	pending := s.fixtures["pending-approve"]
	detail = execute(enrollment.Request{Action: enrollment.Detail, TargetID: pending.SubmissionID})
	approved := execute(enrollment.Request{Action: enrollment.Approve, TargetID: pending.SubmissionID, IdempotencyKey: ids.New(), IfMatch: detail.ETag, Confirmed: true, IdentityCheckMethod: "CAMPUS_AUTHORIZED_CHECK", AttestationVersion: enrollment.NoticeVersion, CampusCheckRef: pending.IdentityReference})
	if approved.HTTPStatus != 202 {
		t.Fatal("approval receipt is not asynchronous")
	}
	control(fixtureControl{Command: "processor.mode", Mode: "fail"})
	control(fixtureControl{Command: "worker.step"})
	v = execute(enrollment.Request{Action: enrollment.Detail, TargetID: pending.SubmissionID}).Data.(enrollment.EnrollmentView)
	if v.SubmissionStatus != "FAILED_PROCESSING" || v.ReasonCode != "EXTRACTION_FAILED" || v.TemplateReady || v.PhotoAvailable {
		t.Fatalf("real processing failure=%+v", v)
	}
	control(fixtureControl{Command: "processor.mode", Mode: "success"})
	control(fixtureControl{Command: "clock.advance", Seconds: int64(enrollment.PhotoTTL / time.Second)})
	// Advancing the shared clock expires real sessions too: issue a fresh internal
	// test session rather than stretching production token lifetimes.
	adminSession, err = s.fixtureSession(ctx, s.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	queue = execute(enrollment.Request{Action: enrollment.List, FilterStatus: "PENDING_APPROVAL"}).Data.(enrollment.ListView)
	if len(queue.Items) != 0 || queue.Summary.PendingCount != 0 {
		t.Fatal("clock advance did not run actual expiry reconciliation")
	}
	control(fixtureControl{Command: "account.role", Fixture: "admin", Role: "dosen"})
	_, err = s.service.Execute(ctx, enrollment.Request{Action: enrollment.List, ActorID: s.admin.ID, SessionID: adminSession})
	var denied *enrollment.Error
	if !errors.As(err, &denied) || denied.Code != "FORBIDDEN_ROLE" {
		t.Fatalf("role loss bypassed domain=%v", err)
	}
	control(fixtureControl{Command: "account.role", Fixture: "admin", Role: "admin"})
	control(fixtureControl{Command: "account.status", Fixture: "admin", Status: "suspended"})
	_, err = s.service.Execute(ctx, enrollment.Request{Action: enrollment.List, ActorID: s.admin.ID, SessionID: adminSession})
	if !errors.As(err, &denied) || denied.Code != "UNAUTHORIZED" {
		t.Fatalf("account suspension bypassed domain=%v", err)
	}
	var output bytes.Buffer
	stopped := false
	s.readControls(ctx, strings.NewReader("{\"command\":\"worker.pause\"}\n{\"command\":\"unknown\"}\n"), &output, func() { stopped = true })
	if !stopped {
		t.Fatal("fixture control EOF did not stop process")
	}
	decoder := json.NewDecoder(&output)
	for _, wantOK := range []bool{true, false} {
		var ack map[string]any
		if err := decoder.Decode(&ack); err != nil || ack["ok"] != wantOK || ack["fixture_control"] != true || ack["server_now"] == "" {
			t.Fatalf("control acknowledgement=%v err=%v", ack, err)
		}
	}
}

func TestStagingWebExportNeverFallsBackForAPIOrEscapesRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("synthetic expo index"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.js"), []byte("synthetic script"), 0o600); err != nil {
		t.Fatal(err)
	}
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Real-Router", "true")
		http.Error(w, "actual API denial", http.StatusForbidden)
	})
	handler, err := stagingWebHandler(api, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		url, accept string
		status      int
		body        string
		api         bool
	}{
		{"/", "text/html", 200, "synthetic expo index", false},
		{"/admin/review", "text/html", 200, "synthetic expo index", false},
		{"/app.js", "*/*", 200, "synthetic script", false},
		{"/missing.js", "text/html", 404, "404 page not found", false},
		{"/api/v1/face-enrollment-submissions", "text/html", 403, "actual API denial", true},
		{"/api/v1/nonexistent", "text/html", 403, "actual API denial", true},
		{"/api", "text/html", 403, "actual API denial", true},
		{"/..%5coutside.txt", "text/html", 404, "404 page not found", false},
		{"/C:%5coutside.txt", "text/html", 404, "404 page not found", false},
		{"/..%20/outside.txt", "text/html", 404, "404 page not found", false},
		{"/.../outside.txt", "text/html", 404, "404 page not found", false},
	} {
		r := httptest.NewRequest("GET", test.url, nil)
		r.Header.Set("Accept", test.accept)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status || !strings.Contains(w.Body.String(), test.body) || (w.Header().Get("X-Real-Router") == "true") != test.api {
			t.Fatalf("%s status=%d body=%s", test.url, w.Code, w.Body.String())
		}
	}
	if _, err := stagingWebHandler(api, t.TempDir()); err == nil {
		t.Fatal("missing web export accepted")
	}
	defaultHandler, err := stagingWebHandler(api, "")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	defaultHandler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusForbidden {
		t.Fatal("default handler no longer uses unchanged router")
	}
}
