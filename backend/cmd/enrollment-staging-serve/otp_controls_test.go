package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

func otpBarrierStaging(t *testing.T) *staging {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "otp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// Only delivery metadata is needed; no OTP code, verifier or auth exception
	// exists in this isolated database fixture.
	if _, err := db.Exec(`CREATE TABLE email_otp_challenges (id TEXT PRIMARY KEY, delivery TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	return &staging{db: db}
}

func seedOTPDelivery(t *testing.T, s *staging, id, delivery string) {
	t.Helper()
	if _, err := s.db.Exec(`INSERT INTO email_otp_challenges (id,delivery) VALUES (?,?)`, id, delivery); err != nil {
		t.Fatal(err)
	}
}

func TestOTPWaitRequiresExactChallengeCommittedDelivery(t *testing.T) {
	s := otpBarrierStaging(t)
	id := ids.New()
	seedOTPDelivery(t, s, id, "pending")
	seedOTPDelivery(t, s, ids.New(), "delivered")
	writer, err := database.Open(context.Background(), s.db.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	tx, err := writer.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE email_otp_challenges SET delivery='delivered' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.waitOTPDelivery(ctx, id) }()
	select {
	case err := <-done:
		t.Fatalf("uncommitted or unrelated delivery satisfied barrier: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("committed requested delivery did not satisfy barrier: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("barrier did not observe delivery commit")
	}
}

func TestOTPWaitCancellationAndCallerDeadline(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "cancel"
		if deadline {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			s := otpBarrierStaging(t)
			id := ids.New()
			seedOTPDelivery(t, s, id, "pending")
			ctx, cancel := context.WithCancel(context.Background())
			want := context.Canceled
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 30*time.Millisecond)
				want = context.DeadlineExceeded
			}
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- s.waitOTPDelivery(ctx, id) }()
			if !deadline {
				// Cancel an in-flight pending barrier, not merely a pre-canceled call.
				time.Sleep(30 * time.Millisecond)
				cancel()
			}
			select {
			case err := <-done:
				if !errors.Is(err, want) {
					t.Fatalf("barrier cancellation error=%v, want %v", err, want)
				}
			case <-time.After(time.Second):
				t.Fatal("barrier ignored cancellation")
			}
			var delivery string
			if err := s.db.QueryRow(`SELECT delivery FROM email_otp_challenges WHERE id=?`, id).Scan(&delivery); err != nil || delivery != "pending" {
				t.Fatalf("read-only barrier changed delivery=%q err=%v", delivery, err)
			}
		})
	}
}

func TestOTPWaitRejectsNoncanonicalAndUndeliveredChallenges(t *testing.T) {
	s := otpBarrierStaging(t)
	id := "abcdef12-3456-4789-abcd-0123456789ab"
	seedOTPDelivery(t, s, id, "delivered")
	for _, invalid := range []string{"", strings.ToUpper(id), " " + id, "00000000-0000-0000-0000-000000000000", id + "extra"} {
		if err := s.waitOTPDelivery(context.Background(), invalid); err == nil {
			t.Fatalf("accepted noncanonical challenge %q", invalid)
		}
	}
	if err := s.waitOTPDelivery(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	failed := ids.New()
	seedOTPDelivery(t, s, failed, "failed")
	for _, unavailable := range []string{failed, ids.New()} {
		if err := s.waitOTPDelivery(context.Background(), unavailable); err == nil {
			t.Fatal("unavailable challenge satisfied readiness barrier")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.waitOTPDelivery(ctx, id); !errors.Is(err, context.Canceled) {
		t.Fatalf("delivered challenge ignored canceled caller: %v", err)
	}
}

func TestOTPControlsRejectSensitiveEchoAndBoundInput(t *testing.T) {
	s := otpBarrierStaging(t)
	id := ids.New()
	seedOTPDelivery(t, s, id, "delivered")
	input := "{\"command\":\"otp.wait\",\"challenge_id\":\"" + id + "\"}\n" +
		"{\"command\":\"otp.wait\",\"challenge_id\":\"" + id + "\",\"code\":\"12345678\"}\n" +
		"{\"command\":\"secret-token-not-a-command\"}\n"
	var output bytes.Buffer
	stopped := false
	s.readOTPControls(context.Background(), strings.NewReader(input), &output, func() { stopped = true })
	if !stopped {
		t.Fatal("EOF did not stop disposable process")
	}
	for _, sensitive := range []string{id, "12345678", "secret-token-not-a-command", "challenge_id"} {
		if strings.Contains(output.String(), sensitive) {
			t.Fatal("control acknowledgment echoed sensitive caller content")
		}
	}
	decoder := json.NewDecoder(&output)
	for _, wantOK := range []bool{true, false, false} {
		var ack map[string]any
		if err := decoder.Decode(&ack); err != nil || ack["fixture_control"] != true || ack["ok"] != wantOK {
			t.Fatalf("unexpected control acknowledgment=%v err=%v", ack, err)
		}
		if wantOK && ack["command"] != "otp.wait" {
			t.Fatal("successful barrier acknowledgment omitted command")
		}
	}
	output.Reset()
	stopped = false
	s.readOTPControls(context.Background(), strings.NewReader(strings.Repeat("x", 4097)+"\n"), &output, func() { stopped = true })
	if !stopped || output.Len() != 0 {
		t.Fatal("oversized JSONL input was not bounded and stopped")
	}
}
