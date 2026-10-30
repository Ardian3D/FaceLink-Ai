package emailotp

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
)

type sentCode struct{ email, code string }
type fakeSender struct {
	mu      sync.Mutex
	sent    []sentCode
	fail    bool
	block   <-chan struct{}
	started chan struct{}
}

func (f *fakeSender) Send(ctx context.Context, email, code string) error {
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentCode{email, code})
	if f.fail {
		return errors.New("smtp failed")
	}
	return nil
}
func (f *fakeSender) latest(t *testing.T) sentCode {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		t.Fatal("no message sent")
	}
	return f.sent[len(f.sent)-1]
}
func (f *fakeSender) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.sent) }

func fixture(t *testing.T, sender *fakeSender) (*Service, *database.DB, *clock.Fake) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "otp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err = migrations.Run(context.Background(), db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC))
	s, err := New(db, clk, []byte(strings.Repeat("k", 32)), sender)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, db, clk
}
func seed(t *testing.T, db *database.DB, email string, role auth.Role, status auth.Status) string {
	t.Helper()
	id := ids.New()
	if _, err := db.Exec(`INSERT INTO users(id,email,display_name,role,status) VALUES(?,?,?,?,?)`, id, email, "Invited user", role, status); err != nil {
		t.Fatal(err)
	}
	return id
}

// Taking every queue permit waits for all worker updates without sleeps or polling.
func idle(t *testing.T, s *Service) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for range queueCapacity {
		select {
		case s.slots <- struct{}{}:
		case <-deadline.C:
			t.Fatal("worker did not drain")
		}
	}
	for range queueCapacity {
		<-s.slots
	}
}
func request(t *testing.T, s *Service, email string) Challenge {
	t.Helper()
	c, err := s.Request(context.Background(), email, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if !ValidChallengeID(c.ChallengeID) || c.ExpiresIn != 300 || c.RetryAfter != 60 {
		t.Fatal("invalid public challenge metadata")
	}
	idle(t, s)
	return c
}
func verify(t *testing.T, s *Service, c Challenge, code string, want error) auth.User {
	t.Helper()
	u, err := s.Verify(context.Background(), c.ChallengeID, code, "192.0.2.1")
	if !errors.Is(err, want) {
		t.Fatalf("verify error=%v, want %v", err, want)
	}
	return u
}

func TestInvitationRolesActivateWithoutProvisioning(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleAdmin, auth.RoleDosen, auth.RoleMahasiswa} {
		t.Run(string(role), func(t *testing.T) {
			f := &fakeSender{}
			s, db, _ := fixture(t, f)
			id := seed(t, db, "person@example.test", role, auth.StatusInvited)
			c := request(t, s, " PERSON@EXAMPLE.TEST ")
			sent := f.latest(t)
			if sent.email != "person@example.test" || !validCode(sent.code) {
				t.Fatal("invalid recipient or code format")
			}
			u := verify(t, s, c, sent.code, nil)
			if u.ID != id || u.Role != role || u.Status != auth.StatusActive {
				t.Fatal("database role or activation lost")
			}
			verify(t, s, c, sent.code, ErrInvalid)
			var identities, users int
			if err := db.QueryRow(`SELECT COUNT(*) FROM email_verified_identities WHERE user_id=? AND email_binding=?`, id, sent.email).Scan(&identities); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
				t.Fatal(err)
			}
			if identities != 1 || users != 1 {
				t.Fatal("identity missing or unexpected account provisioned")
			}
			var raw string
			if err := db.QueryRow(`SELECT hex(verifier) FROM email_otp_challenges WHERE id=?`, c.ChallengeID).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if len(raw) != 64 {
				t.Fatal("verifier must be HMAC-SHA256")
			}
		})
	}
}

func TestIneligibleRequestsUseDecoys(t *testing.T) {
	f := &fakeSender{}
	s, db, _ := fixture(t, f)
	seed(t, db, "suspended@example.test", auth.RoleAdmin, auth.StatusSuspended)
	for _, email := range []string{"absent@example.test", "suspended@example.test"} {
		c := request(t, s, email)
		verify(t, s, c, "00000000", ErrInvalid)
		var status string
		if err := db.QueryRow(`SELECT delivery FROM email_otp_challenges WHERE id=?`, c.ChallengeID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "failed" {
			t.Fatal("decoy did not traverse worker")
		}
	}
	if f.count() != 0 {
		t.Fatal("ineligible recipient was sent a message")
	}
	var users int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 1 {
		t.Fatal("request provisioned an unknown account")
	}
}

func TestCooldownDoesNotSupersedeButAcceptedResendDoes(t *testing.T) {
	f := &fakeSender{}
	s, db, clk := fixture(t, f)
	seed(t, db, "person@example.test", auth.RoleDosen, auth.StatusActive)
	first := request(t, s, "person@example.test")
	firstCode := f.latest(t).code
	decoy := request(t, s, "person@example.test")
	if f.count() != 1 || first.ChallengeID == decoy.ChallengeID {
		t.Fatal("cooldown must produce a fresh unsent decoy")
	}
	verify(t, s, decoy, firstCode, ErrInvalid)
	var superseded int
	if err := db.QueryRow(`SELECT superseded FROM email_otp_challenges WHERE id=?`, first.ChallengeID).Scan(&superseded); err != nil {
		t.Fatal(err)
	}
	if superseded != 0 {
		t.Fatal("cooldown cancelled the usable challenge")
	}
	clk.Advance(Cooldown)
	second := request(t, s, "person@example.test")
	secondCode := f.latest(t).code
	verify(t, s, first, firstCode, ErrInvalid)
	verify(t, s, second, secondCode, nil)
}

func TestExpiryAttemptsAndMalformedCodes(t *testing.T) {
	for _, mode := range []string{"expiry", "malformed", "wrong"} {
		t.Run(mode, func(t *testing.T) {
			f := &fakeSender{}
			s, db, clk := fixture(t, f)
			seed(t, db, "person@example.test", auth.RoleDosen, auth.StatusActive)
			c := request(t, s, "person@example.test")
			code := f.latest(t).code
			if mode == "expiry" {
				clk.Advance(TTL)
				verify(t, s, c, code, ErrInvalid)
				return
			}
			bad, want := "bad", ErrValidation
			if mode == "wrong" {
				bad = "00000000"
				if code == bad {
					bad = "11111111"
				}
				want = ErrInvalid
			}
			for range 5 {
				verify(t, s, c, bad, want)
			}
			verify(t, s, c, code, ErrInvalid)
			var attempts int
			if err := db.QueryRow(`SELECT attempts FROM email_otp_challenges WHERE id=?`, c.ChallengeID).Scan(&attempts); err != nil {
				t.Fatal(err)
			}
			if attempts != 5 {
				t.Fatal("attempt cap was bypassed")
			}
		})
	}
}

func TestConcurrentConsumeHasOneWinner(t *testing.T) {
	f := &fakeSender{}
	s, db, _ := fixture(t, f)
	seed(t, db, "person@example.test", auth.RoleMahasiswa, auth.StatusInvited)
	c := request(t, s, "person@example.test")
	code := f.latest(t).code
	var won atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Verify(context.Background(), c.ChallengeID, code, "192.0.2.1")
			if err == nil {
				won.Add(1)
			} else if !errors.Is(err, ErrInvalid) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if won.Load() != 1 {
		t.Fatalf("consume winners=%d", won.Load())
	}
}

func TestIdentityBindingAndStatusCannotBeInherited(t *testing.T) {
	for _, change := range []string{"email", "suspend", "role", "replacement"} {
		t.Run(change, func(t *testing.T) {
			f := &fakeSender{}
			s, db, _ := fixture(t, f)
			id := seed(t, db, "person@example.test", auth.RoleAdmin, auth.StatusInvited)
			c := request(t, s, "person@example.test")
			var err error
			switch change {
			case "email":
				_, err = db.Exec(`UPDATE users SET email='changed@example.test' WHERE id=?`, id)
			case "suspend":
				_, err = db.Exec(`UPDATE users SET status='suspended' WHERE id=?`, id)
			case "role":
				_, err = db.Exec(`INSERT INTO roles(name,description) VALUES('unexpected','invalid human role')`)
				if err == nil {
					_, err = db.Exec(`UPDATE users SET role='unexpected' WHERE id=?`, id)
				}
			case "replacement":
				_, err = db.Exec(`DELETE FROM users WHERE id=?`, id)
				if err == nil {
					seed(t, db, "person@example.test", auth.RoleAdmin, auth.StatusInvited)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			verify(t, s, c, f.latest(t).code, ErrInvalid)
		})
	}
}

func TestSMTPFailureAndPendingFailClosed(t *testing.T) {
	gate := make(chan struct{})
	f := &fakeSender{block: gate, started: make(chan struct{}, 1), fail: true}
	s, db, _ := fixture(t, f)
	seed(t, db, "person@example.test", auth.RoleDosen, auth.StatusInvited)
	c, err := s.Request(context.Background(), "person@example.test", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	<-f.started
	verify(t, s, c, "00000000", ErrInvalid)
	close(gate)
	idle(t, s)
	verify(t, s, c, f.latest(t).code, ErrInvalid)
	var status string
	if err := db.QueryRow(`SELECT status FROM users`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "invited" {
		t.Fatal("SMTP failure activated the account")
	}
}

func TestDelayedSupersededCodeAndUniformQueueCapacity(t *testing.T) {
	gate := make(chan struct{})
	f := &fakeSender{block: gate, started: make(chan struct{}, 1)}
	s, db, clk := fixture(t, f)
	seed(t, db, "person@example.test", auth.RoleDosen, auth.StatusActive)
	first, err := s.Request(context.Background(), "person@example.test", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	<-f.started
	clk.Advance(Cooldown)
	latest, err := s.Request(context.Background(), "person@example.test", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	// Decoys occupy exactly the same bounded queue as eligible jobs.
	for i := range queueCapacity - 2 {
		_, err = s.Request(context.Background(), "absent@example.test", "192.0.2."+strings.Repeat("1", i+2))
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, email := range []string{"person@example.test", "unknown@example.test"} {
		if _, err = s.Request(context.Background(), email, "192.0.2.200"); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("full queue error=%v", err)
		}
	}
	close(gate)
	idle(t, s)
	f.mu.Lock()
	oldCode, newCode := f.sent[0].code, f.sent[1].code
	f.mu.Unlock()
	verify(t, s, first, oldCode, ErrInvalid)
	verify(t, s, latest, newCode, nil)
}

func TestPersistentRequestLimitsAndEmailDecoyBudget(t *testing.T) {
	f := &fakeSender{}
	s, db, clk := fixture(t, f)
	seed(t, db, "person@example.test", auth.RoleDosen, auth.StatusActive)
	var latest Challenge
	for range 5 {
		latest = request(t, s, "person@example.test")
		clk.Advance(Cooldown)
	}
	code := f.latest(t).code
	decoy := request(t, s, "person@example.test")
	if f.count() != 5 {
		t.Fatal("email hourly budget exceeded")
	}
	verify(t, s, decoy, code, ErrInvalid)
	verify(t, s, latest, code, nil)
	// The same counters apply to unknown emails and survive service restart.
	s.Close()
	restarted, err := New(db, clk, []byte(strings.Repeat("k", 32)), f)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restarted.Close)
	for range 24 {
		request(t, restarted, "unknown@example.test")
	}
	_, err = restarted.Request(context.Background(), "another@example.test", "192.0.2.1")
	var rate *RateError
	if !errors.As(err, &rate) || rate.RetryAfter <= 0 {
		t.Fatalf("IP limit did not persist: %v", err)
	}
}

func TestVerifyUnknownIDsStillUsePersistentLimits(t *testing.T) {
	f := &fakeSender{}
	s, db, clk := fixture(t, f)
	for range 60 {
		if _, err := s.Verify(context.Background(), ids.New(), "00000000", "192.0.2.1"); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	_, err := s.Verify(context.Background(), ids.New(), "bad", "192.0.2.1")
	var rate *RateError
	if !errors.As(err, &rate) {
		t.Fatalf("unknown UUID IP limit missing: %v", err)
	}
	s.Close()
	r, err := New(db, clk, []byte(strings.Repeat("k", 32)), f)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	if _, err = r.Verify(context.Background(), ids.New(), "00000000", "192.0.2.1"); !errors.As(err, &rate) {
		t.Fatal("verify counters lost on restart")
	}
	clk.Advance(10 * time.Minute)
	if _, err = r.Verify(context.Background(), ids.New(), "00000000", "192.0.2.1"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestGlobalAndAccountLimits(t *testing.T) {
	t.Run("request-global", func(t *testing.T) {
		s, _, _ := fixture(t, &fakeSender{})
		for i := range 200 {
			if _, err := s.Request(context.Background(), "unknown@example.test", string(rune(i+1000))); err != nil {
				t.Fatal(err)
			}
			idle(t, s)
		}
		_, err := s.Request(context.Background(), "unknown@example.test", "new-ip")
		var rate *RateError
		if !errors.As(err, &rate) {
			t.Fatalf("global request limit missing: %v", err)
		}
	})
	t.Run("verify-global", func(t *testing.T) {
		s, _, _ := fixture(t, &fakeSender{})
		for i := range 600 {
			if _, err := s.Verify(context.Background(), ids.New(), "00000000", string(rune(i+1000))); !errors.Is(err, ErrInvalid) {
				t.Fatal(err)
			}
		}
		_, err := s.Verify(context.Background(), ids.New(), "00000000", "new-ip")
		var rate *RateError
		if !errors.As(err, &rate) {
			t.Fatalf("global verify limit missing: %v", err)
		}
	})
	t.Run("verify-email", func(t *testing.T) {
		f := &fakeSender{}
		s, db, clk := fixture(t, f)
		seed(t, db, "person@example.test", auth.RoleDosen, auth.StatusActive)
		for range 4 {
			c := request(t, s, "person@example.test")
			for range 5 {
				verify(t, s, c, "bad", ErrValidation)
			}
			clk.Advance(Cooldown)
		}
		c := request(t, s, "person@example.test")
		_, err := s.Verify(context.Background(), c.ChallengeID, f.latest(t).code, "192.0.2.2")
		var rate *RateError
		if !errors.As(err, &rate) {
			t.Fatalf("account limit missing: %v", err)
		}
	})
}

func TestKeyRotationAndCancellationRollback(t *testing.T) {
	f := &fakeSender{}
	s, db, clk := fixture(t, f)
	seed(t, db, "person@example.test", auth.RoleDosen, auth.StatusActive)
	c := request(t, s, "person@example.test")
	s.Close()
	r, err := New(db, clk, []byte(strings.Repeat("x", 32)), f)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	verify(t, r, c, f.latest(t).code, ErrInvalid)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = r.Request(ctx, "another@example.test", "192.0.2.1"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	request(t, r, "another@example.test")
}

func TestValidationAndConfiguration(t *testing.T) {
	for _, value := range []string{"", "name <a@example.test>", "a@example.test,b@example.test", "a@example.test\r\nBcc:x@example.test", strings.Repeat("a", 255) + "@example.test"} {
		if _, err := NormalizeEmail(value); !errors.Is(err, ErrValidation) {
			t.Fatal("invalid email accepted")
		}
	}
	if _, err := New(nil, nil, make([]byte, 32), &fakeSender{}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("missing database accepted")
	}
	f := &fakeSender{}
	s, db, clk := fixture(t, f)
	for _, size := range []int{0, 31, 33} {
		if _, err := New(db, clk, make([]byte, size), f); !errors.Is(err, ErrUnavailable) {
			t.Fatal("invalid key accepted")
		}
	}
	s.Close()
	if _, err := s.Request(context.Background(), "a@example.test", "ip"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("closed service accepted request")
	}
}
