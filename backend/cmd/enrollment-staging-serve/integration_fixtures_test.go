package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentcv"
)

func TestIntegrationFixturesRequireConcretePythonProcessor(t *testing.T) {
	for _, processor := range []enrollment.Processor{nil, syntheticProcessor{}} {
		if s, err := newStaging(context.Background(), "unused", false, true, processor); err == nil {
			s.close()
			t.Fatal("integration accepted a missing or synthetic processor")
		}
	}
	if s, err := newStaging(context.Background(), "unused", true, true, &enrollmentcv.Processor{}); err == nil {
		s.close()
		t.Fatal("integration accepted deterministic Admin seeds")
	}
}

// These tests inspect provisioning/control isolation only. No inference occurs;
// they are not evidence of a real-model processing PASS. The combined TS chain
// supplies fully configured Python/models and exercises the actual processor.
func TestIntegrationFixturesSeedOnlyNormalOTPAccountsAndRoster(t *testing.T) {
	ctx := context.Background()
	s, err := newStaging(ctx, filepath.Join("..", "..", "migrations"), false, true, &enrollmentcv.Processor{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	if s.integration == nil || s.control == nil || !s.control.workerPaused() {
		t.Fatal("integration controls must start with the worker paused")
	}
	if !fixtureUUID(s.integration.traceID) {
		t.Fatal("missing run correlation UUID")
	}
	if s.migration.CurrentVersion != 8 {
		t.Fatal("integration must use isolated schema 8")
	}
	var journal string
	if err = s.db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&journal); err != nil || journal != "wal" {
		t.Fatalf("expected disposable WAL database, journal=%q err=%v", journal, err)
	}
	for _, table := range []string{"session_tokens", "consent_records", "face_enrollment_submissions", "face_templates", "face_enrollments", "enrollment_processing_jobs"} {
		var count int
		if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("integration preseeded %s", table)
		}
	}
	if len(s.fixtures) != 4 {
		t.Fatalf("unexpected fixture account count %d", len(s.fixtures))
	}
	for _, name := range []string{"admin", "student", "reviewer2", "lecturer"} {
		fixture := s.fixtures[name]
		if !fixtureUUID(fixture.UserID) || fixture.Email == "" || fixture.SubmissionID != "" || fixture.Status != "" {
			t.Fatalf("invalid account-only fixture %s", name)
		}
	}
	if s.fixtures["student"].PersonID != s.person.ID || s.fixtures["student"].IdentityReference != s.person.IdentityReference || !fixtureUUID(s.person.IdentityReference) {
		t.Fatal("student fixture must expose its actual synthetic UUID roster identity")
	}
	var members int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM class_memberships WHERE person_id=? AND class_id=? AND left_at IS NULL`, s.person.ID, s.class.ID).Scan(&members); err != nil || members != 1 {
		t.Fatal("student roster membership missing")
	}
}

func TestIntegrationControlsAllowOnlySafeRealServiceOperationsAndStopAtEOF(t *testing.T) {
	ctx := context.Background()
	s, err := newStaging(ctx, filepath.Join("..", "..", "migrations"), false, true, &enrollmentcv.Processor{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	input := strings.NewReader("{\"command\":\"worker.step\"}\n{\"command\":\"reconcile\"}\n{\"command\":\"clock.advance\",\"seconds\":1}\n{\"command\":\"processor.mode\"}\n{\"command\":\"snapshot\",\"submission_id\":\"not-a-uuid\"}\n")
	var output bytes.Buffer
	stopped := false
	s.readIntegrationControls(ctx, input, &output, func() { stopped = true })
	if !stopped {
		t.Fatal("stdin EOF must stop the owned process")
	}
	decoder := json.NewDecoder(&output)
	for i := range 5 {
		var ack struct {
			FixtureControl bool   `json:"fixture_control"`
			OK             bool   `json:"ok"`
			TraceID        string `json:"trace_id"`
			Error          string `json:"error"`
		}
		if err = decoder.Decode(&ack); err != nil {
			t.Fatal(err)
		}
		if !ack.FixtureControl || ack.TraceID != s.integration.traceID || ack.OK != (i < 3) {
			t.Fatalf("unexpected control result %d", i)
		}
		if i >= 3 && ack.Error != "fixture control rejected" {
			t.Fatal("unsafe control error")
		}
	}
	if len(s.integration.observed) != 0 {
		t.Fatal("empty worker step must not invoke inference")
	}
	if !s.control.workerPaused() {
		t.Fatal("integration control resumed automatic worker")
	}
}
