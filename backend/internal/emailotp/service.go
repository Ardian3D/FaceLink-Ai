// Package emailotp implements bounded, invitation-only email authentication.
package emailotp

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"math/big"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

const (
	TTL           = 5 * time.Minute
	Cooldown      = time.Minute
	queueCapacity = 64
)

var (
	ErrInvalid     = errors.New("OTP_INVALID")
	ErrValidation  = errors.New("VALIDATION_FAILED")
	ErrUnavailable = errors.New("AUTH_UNAVAILABLE")
)

// RateError reports only the public retry deadline, never the limiting identity.
type RateError struct{ RetryAfter int }

func (e *RateError) Error() string { return "RATE_LIMITED" }

type Sender interface {
	Send(context.Context, string, string) error
}

type Challenge struct {
	ChallengeID string `json:"challenge_id"`
	ExpiresIn   int    `json:"expires_in"`
	RetryAfter  int    `json:"retry_after"`
	Message     string `json:"message"`
}

type job struct{ id, recipient, code string }

type Service struct {
	db     *database.DB
	clk    clock.Clock
	key    [32]byte
	sender Sender
	mu     sync.Mutex
	closed bool
	queue  chan job
	slots  chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func New(db *database.DB, clk clock.Clock, key []byte, sender Sender) (*Service, error) {
	if db == nil || sender == nil || len(key) != 32 {
		return nil, ErrUnavailable
	}
	if clk == nil {
		clk = clock.Real{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{db: db, clk: clk, sender: sender, queue: make(chan job, queueCapacity), slots: make(chan struct{}, queueCapacity), ctx: ctx, cancel: cancel, done: make(chan struct{})}
	copy(s.key[:], key)
	go s.worker()
	return s, nil
}

func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	<-s.done
}

// NormalizeEmail rejects display names, lists, header injection and oversized addresses.
func NormalizeEmail(value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", ErrValidation
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) == 0 || len(value) > 254 {
		return "", ErrValidation
	}
	a, err := mail.ParseAddress(value)
	if err != nil || a.Name != "" || a.Address != value || !strings.Contains(value, "@") {
		return "", ErrValidation
	}
	return value, nil
}

func ValidChallengeID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i := range len(id) {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if id[i] != '-' {
				return false
			}
			continue
		}
		c := id[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func validCode(code string) bool {
	if len(code) != 8 {
		return false
	}
	for i := range code {
		if code[i] < '0' || code[i] > '9' {
			return false
		}
	}
	return true
}

func (s *Service) digest(parts ...string) []byte {
	h := hmac.New(sha256.New, s.key[:])
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum(nil)
}
func (s *Service) bucketKey(purpose, value string) string {
	return hex.EncodeToString(s.digest("facelink/email-otp/rate/v1", purpose, value))
}
func (s *Service) verifier(id, userID, email, code string) []byte {
	return s.digest("facelink/email-otp/challenge/v1", id, userID, email, code)
}

// transaction always rolls back with an independent context, even after cancellation.
func (s *Service) transaction(ctx context.Context, fn func(*sql.Conn) error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(cleanup, "ROLLBACK")
	}()
	if err = fn(conn); err != nil {
		return ErrUnavailable
	}
	if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
		return ErrUnavailable
	}
	return nil
}

// take uses fixed windows, except cooldown whose expiry is relative to its first use.
func (s *Service) take(ctx context.Context, c *sql.Conn, purpose, value string, limit int, width time.Duration, cooldown bool) (bool, int, error) {
	now := s.clk.Now().UTC().Unix()
	key := s.bucketKey(purpose, value)
	var count int
	var expires int64
	err := c.QueryRowContext(ctx, `SELECT count, expires_at FROM email_otp_rate_buckets WHERE bucket_key = ?`, key).Scan(&count, &expires)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, 0, err
	}
	if err == nil && expires > now && count >= limit {
		return false, int(expires - now), nil
	}
	if expires <= now {
		count = 0
		seconds := int64(width / time.Second)
		expires = (now/seconds + 1) * seconds
		if cooldown {
			expires = now + seconds
		}
	}
	_, err = c.ExecContext(ctx, `INSERT INTO email_otp_rate_buckets(bucket_key,count,expires_at) VALUES(?,?,?) ON CONFLICT(bucket_key) DO UPDATE SET count=excluded.count, expires_at=excluded.expires_at`, key, count+1, expires)
	return true, 0, err
}

func (s *Service) cleanup(ctx context.Context, c *sql.Conn) error {
	now := s.clk.Now().UTC().Unix()
	if _, err := c.ExecContext(ctx, `DELETE FROM email_otp_challenges WHERE id IN (SELECT id FROM email_otp_challenges WHERE expires_at <= ? ORDER BY expires_at LIMIT 128)`, now); err != nil {
		return err
	}
	_, err := c.ExecContext(ctx, `DELETE FROM email_otp_rate_buckets WHERE bucket_key IN (SELECT bucket_key FROM email_otp_rate_buckets WHERE expires_at <= ? ORDER BY expires_at LIMIT 128)`, now)
	return err
}

func (s *Service) Request(ctx context.Context, email, ip string) (Challenge, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return Challenge{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Challenge{}, ErrUnavailable
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return Challenge{}, ErrUnavailable
	}
	enqueued := false
	defer func() {
		if !enqueued {
			<-s.slots
		}
	}()
	value, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		return Challenge{}, ErrUnavailable
	}
	code := value.String()
	code = strings.Repeat("0", 8-len(code)) + code
	id := ids.New()
	emailKey := s.bucketKey("email", email)
	var outcome error
	queued := job{id: id}
	err = s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.cleanup(ctx, c); err != nil {
			return err
		}
		for _, limit := range []struct {
			purpose, value string
			max            int
		}{{"request-global", "global", 200}, {"request-ip", ip, 30}} {
			ok, retry, err := s.take(ctx, c, limit.purpose, limit.value, limit.max, time.Hour, false)
			if err != nil {
				return err
			}
			if !ok {
				outcome = &RateError{retry}
				return nil
			}
		}
		allowed, _, err := s.take(ctx, c, "request-cooldown", email, 1, Cooldown, true)
		if err != nil {
			return err
		}
		if allowed {
			allowed, _, err = s.take(ctx, c, "request-email", email, 5, time.Hour, false)
			if err != nil {
				return err
			}
		}
		var userID, binding string
		if allowed {
			var role, status string
			err := c.QueryRowContext(ctx, `SELECT id, email, role, status FROM users WHERE email = ?`, email).Scan(&userID, &binding, &role, &status)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if !auth.Role(role).Valid() || status != string(auth.StatusInvited) && status != string(auth.StatusActive) {
				userID, binding = "", ""
			}
			if _, err := c.ExecContext(ctx, `UPDATE email_otp_challenges SET superseded=1 WHERE email_key=? AND superseded=0`, emailKey); err != nil {
				return err
			}
		}
		var uid any
		if userID != "" {
			uid = userID
			queued.recipient, queued.code = binding, code
		}
		now := s.clk.Now().UTC()
		_, err = c.ExecContext(ctx, `INSERT INTO email_otp_challenges(id,email_key,user_id,email_binding,verifier,delivery,created_at,expires_at) VALUES(?,?,?,?,?,'pending',?,?)`, id, emailKey, uid, binding, s.verifier(id, userID, binding, code), now.Unix(), now.Add(TTL).Unix())
		return err
	})
	if err != nil {
		return Challenge{}, err
	}
	if outcome != nil {
		return Challenge{}, outcome
	}
	s.queue <- queued
	enqueued = true
	return Challenge{ChallengeID: id, ExpiresIn: 300, RetryAfter: 60, Message: "Jika akun memenuhi syarat, kode akan dikirim. Periksa kotak masuk dan spam."}, nil
}

func (s *Service) Verify(ctx context.Context, challengeID, code, ip string) (auth.User, error) {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return auth.User{}, ErrUnavailable
	}
	challengeID = strings.ToLower(challengeID)
	outcome := error(ErrInvalid)
	var user auth.User
	err := s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.cleanup(ctx, c); err != nil {
			return err
		}
		for _, limit := range []struct {
			purpose, value string
			max            int
		}{{"verify-global", "global", 600}, {"verify-ip", ip, 60}} {
			ok, retry, err := s.take(ctx, c, limit.purpose, limit.value, limit.max, 10*time.Minute, false)
			if err != nil {
				return err
			}
			if !ok {
				outcome = &RateError{retry}
				return nil
			}
		}
		if !ValidChallengeID(challengeID) {
			outcome = ErrValidation
			return nil
		}
		var emailKey, binding, delivery string
		var uid sql.NullString
		var expected []byte
		var expires int64
		var attempts, consumed, superseded int
		err := c.QueryRowContext(ctx, `SELECT email_key,user_id,email_binding,verifier,delivery,expires_at,attempts,consumed,superseded FROM email_otp_challenges WHERE id=?`, challengeID).Scan(&emailKey, &uid, &binding, &expected, &delivery, &expires, &attempts, &consumed, &superseded)
		if errors.Is(err, sql.ErrNoRows) {
			if !validCode(code) {
				outcome = ErrValidation
			}
			return nil
		}
		if err != nil {
			return err
		}
		ok, retry, err := s.take(ctx, c, "verify-email", emailKey, 20, 10*time.Minute, false)
		if err != nil {
			return err
		}
		if !ok {
			outcome = &RateError{retry}
			return nil
		}
		if attempts >= 5 {
			if !validCode(code) {
				outcome = ErrValidation
			}
			return nil
		}
		if _, err := c.ExecContext(ctx, `UPDATE email_otp_challenges SET attempts=attempts+1 WHERE id=?`, challengeID); err != nil {
			return err
		}
		if !validCode(code) {
			outcome = ErrValidation
			return nil
		}
		actual := s.verifier(challengeID, uid.String, binding, code)
		matches := subtle.ConstantTimeCompare(expected, actual) == 1
		if !matches || !uid.Valid || consumed != 0 || superseded != 0 || delivery != "delivered" || expires <= s.clk.Now().UTC().Unix() {
			return nil
		}
		var role, status, created, updated string
		var createdBy, lastLogin sql.NullString
		err = c.QueryRowContext(ctx, `SELECT id,email,display_name,role,status,created_by,last_login_at,created_at,updated_at FROM users WHERE id=?`, uid.String).Scan(&user.ID, &user.Email, &user.DisplayName, &role, &status, &createdBy, &lastLogin, &created, &updated)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if user.Email != binding || !auth.Role(role).Valid() || status != string(auth.StatusInvited) && status != string(auth.StatusActive) {
			return nil
		}
		now := s.clk.Now().UTC()
		stamp := audit.FormatWaktu(now)
		if _, err := c.ExecContext(ctx, `INSERT INTO email_verified_identities(user_id,email_binding,verified_at) VALUES(?,?,?) ON CONFLICT(user_id,email_binding) DO NOTHING`, user.ID, binding, stamp); err != nil {
			return err
		}
		if _, err := c.ExecContext(ctx, `UPDATE users SET status='active',last_login_at=?,updated_at=? WHERE id=?`, stamp, stamp, user.ID); err != nil {
			return err
		}
		if _, err := c.ExecContext(ctx, `UPDATE email_otp_challenges SET consumed=1 WHERE id=?`, challengeID); err != nil {
			return err
		}
		user.Role, user.Status, user.CreatedBy = auth.Role(role), auth.StatusActive, createdBy.String
		user.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return err
		}
		user.UpdatedAt, user.LastLoginAt = now, &now
		outcome = nil
		return nil
	})
	if err != nil {
		return auth.User{}, err
	}
	if outcome != nil {
		return auth.User{}, outcome
	}
	return user, nil
}

func (s *Service) worker() {
	defer close(s.done)
	defer func() {
		for {
			select {
			case <-s.queue:
				<-s.slots
			default:
				return
			}
		}
	}()
	for {
		select {
		case <-s.ctx.Done():
			return
		case j := <-s.queue:
			if s.ctx.Err() != nil {
				<-s.slots
				return
			}
			status := "failed"
			ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
			if j.recipient != "" && s.sender.Send(ctx, j.recipient, j.code) == nil && ctx.Err() == nil {
				status = "delivered"
			}
			cancel()
			j.code, j.recipient = "", ""
			update, cancelUpdate := context.WithTimeout(context.Background(), 5*time.Second)
			_, _ = s.db.ExecContext(update, `UPDATE email_otp_challenges SET delivery=? WHERE id=? AND delivery='pending'`, status, j.id)
			cancelUpdate()
			<-s.slots
		}
	}
}
