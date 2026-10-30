package enrollment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

// Service has no background goroutine; only an explicitly injected synthetic harness starts work.
type Service struct {
	db        *database.DB
	clk       clock.Clock
	keys      enrollmentsecure.KeyProvider
	files     *enrollmentsecure.FileStore
	processor Processor
	decode    chan struct{}
	worker    chan struct{}
	ioMu      sync.Mutex
}

func New(db *database.DB, clk clock.Clock, keys enrollmentsecure.KeyProvider, files *enrollmentsecure.FileStore, processor Processor) (*Service, error) {
	if db == nil || db.DB == nil {
		return nil, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	if clk == nil {
		clk = clock.Real{}
	}
	return &Service{db: db, clk: clk, keys: keys, files: files, processor: processor, decode: make(chan struct{}, 2), worker: make(chan struct{}, 1)}, nil
}

func failure(code string, status int) *Error { return &Error{Code: code, Status: status} }
func nowText(t time.Time) string             { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
func etag(n int64) string                    { return `"` + strconv.FormatInt(n, 10) + `"` }
func uuid(s string) bool {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' || s[14] != '4' || !strings.ContainsRune("89abAB", rune(s[19])) {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}
func mutation(a Action) bool {
	return a == Grant || a == Withdraw || a == Submit || a == Approve || a == Reject || a == Retry || a == AssistedWithdraw
}
func adminAction(a Action) bool {
	return a == List || a == Detail || a == Photo || a == Approve || a == Reject || a == Retry || a == AssistedWithdraw
}
func (s *Service) transaction(ctx context.Context, fn func(*sql.Conn) error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return failure("DATABASE_BUSY", 503)
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return failure("DATABASE_BUSY", 503)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(cleanup, "ROLLBACK")
	}()
	if err = fn(conn); err != nil {
		return err
	}
	if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
		return failure("DATABASE_BUSY", 503)
	}
	return nil
}

func (s *Service) authorize(ctx context.Context, c *sql.Conn, r Request) error {
	var role, status, expires, absolute string
	var revoked sql.NullString
	err := c.QueryRowContext(ctx, `SELECT u.role,u.status,t.access_expires_at,t.absolute_expires_at,t.revoked_at FROM users u JOIN session_tokens t ON t.user_id=u.id WHERE u.id=? AND t.id=?`, r.ActorID, r.SessionID).Scan(&role, &status, &expires, &absolute, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return failure("UNAUTHORIZED", 401)
	}
	if err != nil {
		return failure("DATABASE_BUSY", 503)
	}
	a, e1 := time.Parse(time.RFC3339Nano, expires)
	b, e2 := time.Parse(time.RFC3339Nano, absolute)
	if status != "active" || revoked.Valid || e1 != nil || e2 != nil || !s.clk.Now().Before(a) || !s.clk.Now().Before(b) {
		return failure("UNAUTHORIZED", 401)
	}
	want := "mahasiswa"
	if adminAction(r.Action) {
		want = "admin"
	}
	if role != want {
		return failure("FORBIDDEN_ROLE", 403)
	}
	return nil
}

type person struct {
	id, user, name, code string
	epoch, revision      int64
	eligible             bool
}

func (s *Service) person(ctx context.Context, c *sql.Conn, id string, byUser, required bool) (person, error) {
	p := person{}
	where := "p.id=?"
	if byUser {
		where = "p.account_user_id=?"
	}
	err := c.QueryRowContext(ctx, `SELECT p.id,COALESCE(p.account_user_id,''),p.display_name,p.external_code,p.biometric_epoch,p.consent_revision,
	COALESCE((p.status='active' AND p.deleted_at IS NULL AND p.identity_verified_at IS NOT NULL AND p.identity_reference IS NOT NULL AND u.role='mahasiswa' AND u.status='active' AND EXISTS(SELECT 1 FROM class_memberships m JOIN classes k ON k.id=m.class_id WHERE m.person_id=p.id AND m.left_at IS NULL AND k.status='active' AND k.deleted_at IS NULL)),0)
	FROM people p LEFT JOIN users u ON u.id=p.account_user_id WHERE `+where, id).Scan(&p.id, &p.user, &p.name, &p.code, &p.epoch, &p.revision, &p.eligible)
	if errors.Is(err, sql.ErrNoRows) {
		if byUser {
			return p, failure("ACCOUNT_NOT_LINKED", 409)
		}
		return p, failure("NOT_FOUND", 404)
	}
	if err != nil {
		return p, err
	}
	if required && !p.eligible {
		return p, failure("SUBJECT_INELIGIBLE", 409)
	}
	return p, nil
}
func (s *Service) record(ctx context.Context, c *sql.Conn, r Request, action, entity, id, reason string) error {
	actor := audit.ActorUser
	if r.ActorID == "" {
		actor = audit.ActorSystem
	}
	metadata := map[string]string{}
	if reason != "" {
		metadata["reason_code"] = reason
	}
	if err := audit.NewWriter(c, s.clk).Record(ctx, audit.Event{ActorType: actor, ActorID: r.ActorID, Action: action, EntityType: entity, EntityID: id, Result: audit.ResultSuccess, RequestID: r.RequestID, Metadata: metadata}); err != nil {
		return failure("AUDIT_UNAVAILABLE", 503)
	}
	return nil
}

func (s *Service) Execute(ctx context.Context, r Request) (Result, error) {
	if s == nil {
		return Result{}, failure("BIOMETRIC_UNAVAILABLE", 503)
	}
	switch r.Action {
	case Notice, Grant, Withdraw, Status, Submit, List, Detail, Photo, Approve, Reject, Retry, AssistedWithdraw:
	default:
		return Result{}, failure("VALIDATION_FAILED", 400)
	}
	if mutation(r.Action) && !uuid(r.IdempotencyKey) {
		return Result{}, failure("VALIDATION_FAILED", 400)
	}
	if (r.Action == Detail || r.Action == Photo || r.Action == Approve || r.Action == Reject || r.Action == Retry || r.Action == AssistedWithdraw) && !uuid(r.TargetID) {
		return Result{}, failure("VALIDATION_FAILED", 400)
	}
	if r.Action == Submit {
		return s.submit(ctx, r)
	}
	if r.Action == Photo {
		return s.preview(ctx, r)
	}
	if (r.Action == Approve || r.Action == Retry) && r.verifiedPhotoHash == "" {
		return s.verifyApproval(ctx, r)
	}
	var result Result
	err := s.transaction(ctx, func(c *sql.Conn) error {
		if err := s.authorize(ctx, c, r); err != nil {
			return err
		}
		if err := s.maintain(ctx, c); err != nil {
			return err
		}
		var p person
		var err error
		if !adminAction(r.Action) {
			p, err = s.person(ctx, c, r.ActorID, true, r.Action != Withdraw)
			if err != nil {
				return err
			}
		}
		if r.Action == AssistedWithdraw {
			p, err = s.person(ctx, c, r.TargetID, false, false)
			if err != nil {
				return err
			}
		}
		if mutation(r.Action) {
			old, ok, err := s.replay(ctx, c, r, p)
			if err != nil {
				return err
			}
			if ok {
				result = old
				return nil
			}
		}
		if !r.rateCharged {
			if err = s.rate(ctx, c, r); err != nil {
				return err
			}
		}
		switch r.Action {
		case Notice:
			v, err := s.consent(ctx, c, p)
			result = Result{HTTPStatus: 200, Data: v, ETag: etag(p.revision)}
			return err
		case Status:
			v, err := s.view(ctx, c, p, "", false)
			result = Result{HTTPStatus: 200, Data: v, ETag: etag(v.Revision)}
			return err
		case Grant:
			result, err = s.grant(ctx, c, r, p)
		case Withdraw, AssistedWithdraw:
			result, err = s.withdraw(ctx, c, r, p)
		case List:
			result, err = s.list(ctx, c, r)
		case Detail:
			result, err = s.detail(ctx, c, r)
		case Approve, Reject, Retry:
			result, err = s.decide(ctx, c, r)
		}
		if err != nil {
			return err
		}
		if mutation(r.Action) {
			return s.saveReceipt(ctx, c, r, result)
		}
		return nil
	})
	return result, safe(err)
}
func safe(err error) error {
	if err == nil {
		return nil
	}
	var domain *Error
	if errors.As(err, &domain) {
		return domain
	}
	return failure("BIOMETRIC_UNAVAILABLE", 503)
}

func (s *Service) consume(ctx context.Context, c *sql.Conn, scope string, window time.Duration, max int) error {
	now := s.clk.Now().Unix()
	seconds := int64(window / time.Second)
	start := now / seconds * seconds
	var count int
	err := c.QueryRowContext(ctx, `SELECT count FROM enrollment_rate_buckets WHERE scope=? AND window_start=?`, scope, start).Scan(&count)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if count >= max {
		return &Error{Code: "RATE_LIMITED", Status: 429, RetryAfter: int(start + seconds - now)}
	}
	_, err = c.ExecContext(ctx, `INSERT INTO enrollment_rate_buckets(scope,window_start,count,expires_at) VALUES(?,?,1,?) ON CONFLICT(scope,window_start) DO UPDATE SET count=count+1`, scope, start, start+seconds)
	return err
}
func (s *Service) rate(ctx context.Context, c *sql.Conn, r Request) error {
	if _, err := c.ExecContext(ctx, `DELETE FROM enrollment_rate_buckets WHERE expires_at<=?`, s.clk.Now().Unix()); err != nil {
		return err
	}
	if err := s.consume(ctx, c, "actor:"+r.ActorID+":"+string(r.Action), time.Minute, 60); err != nil {
		return err
	}
	if r.Action == Photo {
		return s.consume(ctx, c, "preview:"+r.ActorID, time.Minute, 30)
	}
	if r.Action == Submit {
		for _, b := range []struct {
			scope  string
			period time.Duration
			max    int
		}{{"submit-hour:" + r.ActorID, time.Hour, 3}, {"submit-day:" + r.ActorID, 24 * time.Hour, 10}, {"submit-global", time.Hour, 30}} {
			if err := s.consume(ctx, c, b.scope, b.period, b.max); err != nil {
				return err
			}
		}
	}
	return nil
}

func fingerprint(r Request) string {
	imageHash := sha256.Sum256(r.JPEG)
	r.JPEG = nil
	r.SessionID = ""
	r.RequestID = ""
	r.IdempotencyKey = ""
	body, _ := json.Marshal(struct {
		Version   int
		Request   Request
		ImageHash string
	}{1, r, hex.EncodeToString(imageHash[:])})
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
func scope(r Request) string {
	return "enrollment:" + r.ActorID + ":POST:" + string(r.Action) + ":" + r.TargetID
}
func (s *Service) replay(ctx context.Context, c *sql.Conn, r Request, p person) (Result, bool, error) {
	var hash, body, expiry string
	err := c.QueryRowContext(ctx, `SELECT request_hash,response_body,expires_at FROM idempotency_records WHERE scope=? AND key=?`, scope(r), r.IdempotencyKey).Scan(&hash, &body, &expiry)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	until, err := time.Parse(time.RFC3339Nano, expiry)
	if err != nil {
		return Result{}, false, err
	}
	if !s.clk.Now().Before(until) {
		_, err = c.ExecContext(ctx, `DELETE FROM idempotency_records WHERE scope=? AND key=?`, scope(r), r.IdempotencyKey)
		return Result{}, false, err
	}
	if hash != fingerprint(r) {
		return Result{}, false, failure("IDEMPOTENCY_CONFLICT", 409)
	}
	var receipt MutationReceipt
	if err = json.Unmarshal([]byte(body), &receipt); err != nil {
		return Result{}, false, err
	}
	if adminAction(r.Action) && r.Action != AssistedWithdraw {
		var row submission
		row, err = s.submission(ctx, c, receipt.ResourceID)
		if err != nil {
			return Result{}, false, err
		}
		// Replay must return the already committed receipt even when the subject
		// later became ineligible; the decision path checks eligibility separately.
		p, err = s.person(ctx, c, row.personID, false, false)
		if err != nil {
			return Result{}, false, err
		}
	}
	if r.Action == Grant {
		receipt.Current, err = s.consent(ctx, c, p)
	} else {
		id := ""
		if adminAction(r.Action) && r.Action != AssistedWithdraw {
			id = receipt.ResourceID
		}
		receipt.Current, err = s.view(ctx, c, p, id, adminAction(r.Action))
	}
	if err != nil {
		return Result{}, false, err
	}
	revision := p.revision
	if v, ok := receipt.Current.(EnrollmentView); ok {
		revision = v.Revision
	}
	return Result{HTTPStatus: 200, Data: receipt, ETag: etag(revision), Replay: true}, true, nil
}
func (s *Service) saveReceipt(ctx context.Context, c *sql.Conn, r Request, result Result) error {
	receipt, ok := result.Data.(MutationReceipt)
	if !ok {
		return fmt.Errorf("missing mutation receipt")
	}
	receipt.Current = nil
	body, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	_, err = c.ExecContext(ctx, `INSERT INTO idempotency_records(scope,key,request_hash,response_code,response_body,created_at,expires_at) VALUES(?,?,?,?,?,?,?)`, scope(r), r.IdempotencyKey, fingerprint(r), result.HTTPStatus, string(body), nowText(s.clk.Now()), nowText(s.clk.Now().Add(24*time.Hour)))
	return err
}
func receipt(resource string, revision int64, current any, status int) Result {
	return Result{HTTPStatus: status, ETag: etag(revision), Data: MutationReceipt{OperationID: ids.New(), ResourceID: resource, AppliedRevision: revision, Current: current}}
}
