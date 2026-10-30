package auth

import (
	"context"
	"errors"
	"testing"
)

func TestCore02ConcurrentCloseCommitsOneTransitionAndAudit(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	second := core01SecondStore(t, f.db, f.s)
	session := f.open(t)
	result := core01Race(func() error {
		_, err := f.s.CloseClassSession(ctx, f.lecturer.ID, session.ID, "Penutupan sintetis A")
		return err
	}, func() error {
		_, err := second.CloseClassSession(ctx, f.admin.ID, session.ID, "Penutupan sintetis B")
		return err
	})
	success, conflict := 0, 0
	for _, err := range result {
		if err == nil {
			success++
		} else if errors.Is(err, ErrSessionClosed) {
			conflict++
		} else {
			t.Fatalf("unexpected close result: %v", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("close outcomes: %v", result)
	}
	current, err := f.s.GetClassSession(ctx, f.admin.ID, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != "closed" || current.ClosedAt == nil || f.auditCount(t, "session.closed") != 1 {
		t.Fatalf("close state/audit inconsistent: %+v", current)
	}
	var actor, reason string
	if err := f.db.QueryRowContext(ctx, "SELECT actor_id, reason FROM audit_events WHERE action='session.closed' AND entity_id=?", session.ID).Scan(&actor, &reason); err != nil {
		t.Fatal(err)
	}
	if actor != f.lecturer.ID && actor != f.admin.ID {
		t.Fatalf("invalid closing actor %s", actor)
	}
	// The surviving row and audit must describe the same winning request.
	if current.CloseReason != reason {
		t.Fatalf("stored/audited close reason mismatch %q / %q", current.CloseReason, reason)
	}
}
