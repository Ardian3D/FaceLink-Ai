package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

// This runtime is reachable only from the disposable command's explicit flag.
// Its clock advances all real services together; it never edits deadlines or receipts.
type fixtureRuntime struct {
	mu                  sync.Mutex
	workMu              sync.Mutex
	offset              time.Duration
	paused              bool
	failProcessing      bool
	templateUnavailable bool
	operator            string
}

func (f *fixtureRuntime) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return time.Now().UTC().Add(f.offset)
}
func (f *fixtureRuntime) workerPaused() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.paused
}
func (f *fixtureRuntime) processingFails() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failProcessing
}
func (f *fixtureRuntime) templateIsUnavailable() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.templateUnavailable
}

type adminFixture struct {
	UserID            string `json:"user_id"`
	Email             string `json:"email"`
	PersonID          string `json:"person_id,omitempty"`
	SubmissionID      string `json:"submission_id,omitempty"`
	Status            string `json:"status,omitempty"`
	IdentityReference string `json:"identity_reference,omitempty"`
}

func fixtureJPEG() ([]byte, error) {
	im := image.NewRGBA(image.Rect(0, 0, 112, 112))
	for y := range 112 {
		for x := range 112 {
			im.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 73, A: 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, im, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (s *staging) inviteFixture(ctx context.Context, name string, role auth.Role) (auth.User, error) {
	u, err := s.accounts.InviteUser(ctx, name+".synthetic@campus.example", "Fixture Sintetis "+name, role, s.admin.ID)
	if err != nil {
		return auth.User{}, err
	}
	// Account provisioning mirrors the existing disposable fixture bootstrap;
	// login remains normal email OTP. No production account or auth code is changed.
	if _, err = s.db.ExecContext(ctx, `UPDATE users SET status='active' WHERE id=?`, u.ID); err != nil {
		return auth.User{}, err
	}
	s.fixtures[name] = adminFixture{UserID: u.ID, Email: u.Email}
	return u, nil
}

func (s *staging) fixtureSession(ctx context.Context, userID string) (string, error) {
	pair, err := s.sessions.Issue(ctx, userID, "enr03-disposable-fixture-seed")
	if err != nil {
		return "", err
	}
	// The seed uses a real session but never exports its credentials to UI/stdout.
	pair.AccessToken, pair.RefreshToken = "", ""
	return pair.SessionID, nil
}

func (s *staging) seedAdminFixtures(ctx context.Context) error {
	s.fixtures = map[string]adminFixture{"admin": {UserID: s.admin.ID, Email: s.admin.Email}}
	for _, fixture := range []struct {
		name string
		role auth.Role
	}{{"reviewer2", auth.RoleAdmin}, {"lecturer", auth.RoleDosen}, {"fixture-controller", auth.RoleAdmin}} {
		u, err := s.inviteFixture(ctx, fixture.name, fixture.role)
		if err != nil {
			return err
		}
		if fixture.name == "fixture-controller" {
			s.control.operator = u.ID
		}
	}
	jpegBytes, err := fixtureJPEG()
	if err != nil {
		return err
	}
	defer clear(jpegBytes)
	reviewer := s.fixtures["reviewer2"]
	reviewerSession, err := s.fixtureSession(ctx, reviewer.UserID)
	if err != nil {
		return err
	}
	for _, name := range []string{"pending-approve", "pending-reject", "pending-expiry", "pending-conflict", "failed-processing"} {
		u, err := s.inviteFixture(ctx, name, auth.RoleMahasiswa)
		if err != nil {
			return err
		}
		identityRef := ids.New()
		p, err := s.accounts.CreatePerson(ctx, s.admin.ID, "SYN-ENR03-"+strings.ToUpper(name), "Fixture Sintetis "+name, "", identityRef)
		if err != nil {
			return err
		}
		if _, err = s.accounts.AddClassMember(ctx, s.admin.ID, s.class.ID, p.ID); err != nil {
			return err
		}
		if _, err = s.accounts.LinkPersonAccount(ctx, s.admin.ID, p.ID, u.ID); err != nil {
			return err
		}
		sessionID, err := s.fixtureSession(ctx, u.ID)
		if err != nil {
			return err
		}
		r := enrollment.Request{Action: enrollment.Notice, ActorID: u.ID, SessionID: sessionID, RequestID: ids.New()}
		notice, err := s.service.Execute(ctx, r)
		if err != nil {
			return err
		}
		r.Action, r.IdempotencyKey, r.IfMatch = enrollment.Grant, ids.New(), notice.ETag
		r.Accepted, r.NoticeVersion = true, enrollment.NoticeVersion
		grant, err := s.service.Execute(ctx, r)
		if err != nil {
			return err
		}
		r = enrollment.Request{Action: enrollment.Submit, ActorID: u.ID, SessionID: sessionID, RequestID: ids.New(), IdempotencyKey: ids.New(), ConsentID: grant.Data.(enrollment.MutationReceipt).ResourceID, JPEG: jpegBytes}
		submitted, err := s.service.Execute(ctx, r)
		if err != nil {
			return err
		}
		id := submitted.Data.(enrollment.MutationReceipt).ResourceID
		status := "PENDING_APPROVAL"
		if name == "failed-processing" {
			detail, err := s.service.Execute(ctx, enrollment.Request{Action: enrollment.Detail, ActorID: reviewer.UserID, SessionID: reviewerSession, RequestID: ids.New(), TargetID: id})
			if err != nil {
				return err
			}
			_, err = s.service.Execute(ctx, enrollment.Request{Action: enrollment.Approve, ActorID: reviewer.UserID, SessionID: reviewerSession, RequestID: ids.New(), TargetID: id, IdempotencyKey: ids.New(), IfMatch: detail.ETag, Confirmed: true, IdentityCheckMethod: "CAMPUS_AUTHORIZED_CHECK", AttestationVersion: enrollment.NoticeVersion, CampusCheckRef: identityRef})
			if err != nil {
				return err
			}
			// A real template-key outage causes the domain's conservative retained-
			// ciphertext failure; restore before startup so UI retry is genuinely viable.
			s.control.mu.Lock()
			s.control.templateUnavailable = true
			s.control.mu.Unlock()
			err = s.service.ProcessNext(ctx)
			s.control.mu.Lock()
			s.control.templateUnavailable = false
			s.control.mu.Unlock()
			if err != nil {
				return err
			}
			status = "FAILED_PROCESSING"
		}
		s.fixtures[name] = adminFixture{UserID: u.ID, Email: u.Email, PersonID: p.ID, SubmissionID: id, Status: status, IdentityReference: identityRef}
	}
	return nil
}

type fixtureControl struct {
	Command     string `json:"command"`
	Seconds     int64  `json:"seconds,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Fixture     string `json:"fixture,omitempty"`
	Role        string `json:"role,omitempty"`
	Status      string `json:"status,omitempty"`
	ChallengeID string `json:"challenge_id,omitempty"`
}

func (s *staging) applyControl(ctx context.Context, c fixtureControl) error {
	if c.Command == "otp.wait" {
		return s.waitOTPDelivery(ctx, c.ChallengeID)
	}
	f := s.control
	if f == nil {
		return errors.New("admin fixtures not enabled")
	}
	f.workMu.Lock()
	defer f.workMu.Unlock()
	switch c.Command {
	case "worker.pause", "worker.resume":
		f.mu.Lock()
		f.paused = c.Command == "worker.pause"
		f.mu.Unlock()
	case "worker.step":
		if !f.workerPaused() {
			return errors.New("pause worker before deterministic step")
		}
		return s.service.ProcessNext(ctx)
	case "clock.advance":
		if c.Seconds <= 0 || c.Seconds > int64((7*24*time.Hour)/time.Second) {
			return errors.New("seconds must be 1..604800 (forward only)")
		}
		f.mu.Lock()
		f.offset += time.Duration(c.Seconds) * time.Second
		f.mu.Unlock()
		return s.service.Reconcile(ctx)
	case "processor.mode":
		if c.Mode != "success" && c.Mode != "fail" {
			return errors.New("mode must be success or fail")
		}
		f.mu.Lock()
		f.failProcessing = c.Mode == "fail"
		f.mu.Unlock()
	case "account.role", "account.status":
		if c.Fixture != "admin" && c.Fixture != "reviewer2" {
			return errors.New("account controls permit admin or reviewer2 only")
		}
		u := s.fixtures[c.Fixture]
		if c.Command == "account.role" {
			if c.Role != "admin" && c.Role != "dosen" {
				return errors.New("role must be admin or dosen")
			}
			if _, err := s.accounts.SetRole(ctx, f.operator, u.UserID, auth.Role(c.Role)); err != nil {
				return err
			}
		} else {
			if c.Status != "active" && c.Status != "suspended" {
				return errors.New("status must be active or suspended")
			}
			if _, err := s.accounts.SetStatus(ctx, f.operator, u.UserID, auth.Status(c.Status)); err != nil {
				return err
			}
			if c.Status == "suspended" {
				if _, err := s.sessions.RevokeAllForUser(ctx, u.UserID, "account_suspended"); err != nil {
					return err
				}
			}
		}
		return s.service.Reconcile(ctx)
	default:
		return errors.New("unknown fixture command")
	}
	return nil
}

func (s *staging) readControls(ctx context.Context, input io.Reader, output io.Writer, stop context.CancelFunc) {
	defer stop() // EOF owns the disposable process lifetime; no public control route.
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 4096)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		var c fixtureControl
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		err := decoder.Decode(&c)
		if err == nil {
			var extra any
			if decoder.Decode(&extra) != io.EOF {
				err = errors.New("one JSON object required")
			}
		}
		if err == nil {
			err = s.applyControl(ctx, c)
		}
		ack := map[string]any{"fixture_control": true, "ok": err == nil, "server_now": s.clk.Now().Format(time.RFC3339Nano), "worker_paused": s.control.workerPaused()}
		switch c.Command {
		case "otp.wait", "worker.pause", "worker.resume", "worker.step", "clock.advance", "processor.mode", "account.role", "account.status":
			ack["command"] = c.Command
		}
		if err != nil {
			ack["error"] = "fixture control rejected"
		}
		if encoder.Encode(ack) != nil {
			return
		}
	}
}

// Win32 strips trailing dots/spaces from path components, so a segment such as
// ".. " can escape the export root even after path.Clean. Reject it before stat.
func unsafeStaticSegment(p string) bool {
	for _, segment := range strings.Split(p, "/") {
		if segment != strings.TrimRight(segment, ". ") {
			return true
		}
	}
	return false
}

// stagingWebHandler serves only export files. Every /api path stays on the real
// router, including missing APIs; an SPA fallback must never counterfeit API JSON.
func stagingWebHandler(api http.Handler, root string) (http.Handler, error) {
	if root == "" {
		return api, nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	index, err := os.Stat(filepath.Join(abs, "index.html"))
	if err != nil || !index.Mode().IsRegular() {
		return nil, fmt.Errorf("web-root must contain an Expo export index.html")
	}
	files := http.FileServer(http.Dir(abs))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if strings.ContainsAny(r.URL.Path, "\\:\x00") || unsafeStaticSegment(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		clean := path.Clean("/" + r.URL.Path)
		file := filepath.Join(abs, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
		rel, relErr := filepath.Rel(abs, file)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			http.NotFound(w, r)
			return
		}
		info, statErr := os.Stat(file)
		if statErr == nil && info.Mode().IsRegular() {
			w.Header().Set("Cache-Control", "no-store")
			files.ServeHTTP(w, r)
			return
		}
		if path.Ext(clean) == "" && strings.Contains(r.Header.Get("Accept"), "text/html") {
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, filepath.Join(abs, "index.html"))
			return
		}
		http.NotFound(w, r)
	}), nil
}
