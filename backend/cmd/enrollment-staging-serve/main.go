// Command enrollment-staging-serve runs the synthetic enrollment backend for
// FL-ENR02A/FL-ENR03 staging UI work. It creates a disposable SQLite database, synthetic
// accounts/roster, an ephemeral in-memory KeyProvider and a synthetic processor.
//
// It is not imported by cmd/server and cannot be activated by configuration of
// the production server. Human captures, production keys and the Product Owner
// database are never used. All modes accept stdin-only otp.wait JSONL controls
// to await committed delivery metadata without exposing codes or tokens. Test
// tokens are emitted only when explicitly asked for with loopback-only HTTP.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/jpeg"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/authz"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/emailotp"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentcv"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentfixture"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/httpapi"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/session"
)

// syntheticProcessor is a deterministic fixture. It does not detect, compare or
// identify a face and cannot be enabled on the production command.
type syntheticProcessor struct{ control *fixtureRuntime }

func (syntheticProcessor) Manifest() enrollment.Manifest {
	h := sha256.Sum256([]byte("FL-ENR02A-SYNTHETIC-NOT-A-MODEL"))
	d := hex.EncodeToString(h[:])
	return enrollment.Manifest{
		SyntheticOnly:    true,
		ModelID:          "synthetic-sface",
		ModelVersion:     "test-v1",
		ModelChecksum:    d,
		DetectorID:       "synthetic-yunet",
		DetectorVersion:  "test-v1",
		DetectorChecksum: d,
		PipelineRevision: "synthetic-v1",
	}
}

func (p syntheticProcessor) Process(ctx context.Context, b []byte) (enrollment.ProcessingResult, error) {
	if err := ctx.Err(); err != nil {
		return enrollment.ProcessingResult{}, err
	}
	if p.control != nil && p.control.processingFails() {
		return enrollment.ProcessingResult{}, &enrollment.ProcessingError{Code: "EXTRACTION_FAILED"}
	}
	if _, err := jpeg.Decode(bytes.NewReader(b)); err != nil {
		return enrollment.ProcessingResult{}, err
	}
	v := make([]float32, 128)
	v[0] = 1
	return enrollment.ProcessingResult{Manifest: syntheticProcessor{}.Manifest(), FaceCount: 1, Embedding: v}, nil
}

type stagingKeys struct {
	keys    map[string][]byte
	control *fixtureRuntime
}

func newStagingKeys() (stagingKeys, error) {
	k := stagingKeys{keys: map[string][]byte{}}
	for _, purpose := range []string{enrollmentsecure.PendingPhoto, enrollmentsecure.Template} {
		key := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return stagingKeys{}, err
		}
		k.keys[purpose] = key
	}
	return k, nil
}

func (k stagingKeys) Active(ctx context.Context, purpose string) (string, []byte, error) {
	if ctx.Err() != nil {
		return "", nil, enrollmentsecure.ErrUnavailable
	}
	if k.control != nil && purpose == enrollmentsecure.Template && k.control.templateIsUnavailable() {
		return "", nil, enrollmentsecure.ErrUnavailable
	}
	key, ok := k.keys[purpose]
	if !ok {
		return "", nil, enrollmentsecure.ErrUnavailable
	}
	return "1", bytes.Clone(key), nil
}

func (k stagingKeys) ByVersion(ctx context.Context, purpose, version string) ([]byte, error) {
	if version != "1" {
		return nil, enrollmentsecure.ErrUnavailable
	}
	_, key, err := k.Active(ctx, purpose)
	return key, err
}

type stagingSender struct {
	path string
	mu   sync.Mutex
}

func (s *stagingSender) Send(ctx context.Context, _ string, code string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(code+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

type staging struct {
	root        string
	db          *database.DB
	migration   migrations.Status
	accounts    *auth.Store
	sessions    *session.Store
	service     *enrollment.Service
	otp         *emailotp.Service
	otpPath     string
	admin       auth.User
	student     auth.User
	person      auth.Person
	class       auth.Class
	clk         clock.Clock
	control     *fixtureRuntime
	fixtures    map[string]adminFixture
	integration *integrationRuntime
}

func newStaging(ctx context.Context, baseMigrations string, adminFixtures, integrationFixtures bool, processor enrollment.Processor) (*staging, error) {
	if adminFixtures && processor != nil {
		return nil, errors.New("real CV processor cannot use deterministic Admin seed/control fixtures")
	}
	if integrationFixtures {
		if adminFixtures {
			return nil, errors.New("integration fixtures forbid deterministic Admin fixtures")
		}
		if _, ok := processor.(*enrollmentcv.Processor); !ok {
			return nil, errors.New("integration fixtures require the concrete real Python processor")
		}
	}
	root, err := os.MkdirTemp("", "facelink-enr02a-staging-")
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	fail := func(err error) (*staging, error) { cleanup(); return nil, err }

	db, err := database.Open(ctx, filepath.Join(root, "staging.db"))
	if err != nil {
		return fail(err)
	}
	base, err := filepath.Abs(baseMigrations)
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	combined, err := enrollmentfixture.PrepareMigrations(base, filepath.Join(root, "migrations"))
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	migration, err := migrations.Run(ctx, db, combined)
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	var clk clock.Clock = clock.Real{}
	var control *fixtureRuntime
	if adminFixtures || integrationFixtures {
		control = &fixtureRuntime{paused: true}
		clk = control
	}
	accounts := auth.NewStore(db, clk)
	admin, err := accounts.CreateFirstAdmin(ctx, "admin.synthetic@campus.example", "Admin Sintetis")
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	student, err := accounts.InviteUser(ctx, "student.synthetic@campus.example", "Mahasiswa Sintetis", auth.RoleMahasiswa, admin.ID)
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE users SET status='active' WHERE id IN (?,?)`, admin.ID, student.ID); err != nil {
		_ = db.Close()
		return fail(err)
	}
	identityReference := "roster-synthetic-v1"
	if integrationFixtures {
		identityReference = ids.New()
	}
	person, err := accounts.CreatePerson(ctx, admin.ID, "SYN-ENR02A", "Mahasiswa Sintetis", "", identityReference)
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	class, err := accounts.CreateClass(ctx, admin.ID, "SYN-ENR02A", "Kelas Sintetis")
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	if _, err = accounts.AddClassMember(ctx, admin.ID, class.ID, person.ID); err != nil {
		_ = db.Close()
		return fail(err)
	}
	if _, err = accounts.LinkPersonAccount(ctx, admin.ID, person.ID, student.ID); err != nil {
		_ = db.Close()
		return fail(err)
	}
	keys, err := newStagingKeys()
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	keys.control = control
	files, err := enrollmentsecure.NewFileStore(filepath.Join(root, "photos"))
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	if processor == nil {
		processor = syntheticProcessor{control: control}
	}
	var integration *integrationRuntime
	if integrationFixtures {
		integration = newIntegrationRuntime(processor, db.DB)
		processor = integration
	}
	service, err := enrollment.New(db, clk, keys, files, processor)
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	if err = service.Reconcile(ctx); err != nil {
		_ = db.Close()
		return fail(err)
	}
	otpPath := filepath.Join(root, "otp.txt")
	if err = os.WriteFile(otpPath, nil, 0o600); err != nil {
		_ = db.Close()
		return fail(err)
	}
	otpKey := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, otpKey); err != nil {
		_ = db.Close()
		return fail(err)
	}
	otp, err := emailotp.New(db, clk, otpKey, &stagingSender{path: otpPath})
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	staged := &staging{
		root: root, db: db, migration: migration, accounts: accounts,
		sessions: session.NewStore(db, clk, session.Config{}), service: service,
		otp: otp, otpPath: otpPath, admin: admin, student: student,
		person: person, class: class, clk: clk, control: control,
		integration: integration,
	}
	if adminFixtures {
		if err = staged.seedAdminFixtures(ctx); err != nil {
			staged.close()
			return nil, err
		}
	}
	if integrationFixtures {
		integration.keys = keys
		if err = staged.seedIntegrationAccounts(ctx); err != nil {
			staged.close()
			return nil, err
		}
	}
	return staged, nil
}

func (s *staging) close() {
	if s.otp != nil {
		s.otp.Close()
	}
	if s.db != nil {
		_ = s.db.Close()
	}
	_ = os.RemoveAll(s.root)
}

func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "enrollment-staging-serve: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", "127.0.0.1:8091", "loopback address for the synthetic staging server")
	cert := flag.String("cert", "dev/certs/server.pem", "development TLS certificate")
	key := flag.String("key", "dev/certs/server-key.pem", "development TLS private key")
	baseMigrations := flag.String("migrations", "migrations", "base migration directory")
	insecureHTTP := flag.Bool("insecure-http", false, "plain loopback HTTP for the automated E2E test only")
	testTokens := flag.Bool("test-tokens", false, "emit fixture access tokens in startup JSON (requires -insecure-http)")
	webRoot := flag.String("web-root", "", "serve an Expo web export on the same HTTPS origin (no API bypass)")
	adminFixtures := flag.Bool("admin-fixtures", false, "seed isolated Admin review fixtures and accept stdin-only JSONL controls")
	integrationFixtures := flag.Bool("integration-fixtures", false, "real Python integration with normal OTP accounts and stdin-only safe metadata controls")
	cvPython := flag.String("cv-python", "", "opt-in real-model synthetic-only Python executable (requires cv-engine-dir and cv-model-dir)")
	cvEngine := flag.String("cv-engine-dir", "", "trusted local face-engine source directory")
	cvModels := flag.String("cv-model-dir", "", "trusted local checksum-pinned YuNet/SFace model directory")
	flag.Parse()

	if !loopback(*addr) {
		return fmt.Errorf("enrollment staging server refuses non-loopback address %q", *addr)
	}
	if *testTokens && !*insecureHTTP {
		return errors.New("-test-tokens requires -insecure-http; fixture tokens are never emitted over TLS")
	}
	if *webRoot != "" && *insecureHTTP {
		return errors.New("-web-root requires HTTPS; use the trusted development certificate")
	}
	if *integrationFixtures && (*adminFixtures || *testTokens || *insecureHTTP || *cvPython == "" || *cvEngine == "" || *cvModels == "") {
		return errors.New("-integration-fixtures requires HTTPS and all three cv flags; forbids -admin-fixtures and -test-tokens")
	}
	var processor enrollment.Processor
	if *cvPython != "" || *cvEngine != "" || *cvModels != "" {
		if *cvPython == "" || *cvEngine == "" || *cvModels == "" || *adminFixtures {
			return errors.New("real CV requires all three cv flags and forbids deterministic Admin fixtures")
		}
		var err error
		processor, err = enrollmentcv.New(*cvPython, *cvEngine, *cvModels)
		if err != nil {
			return fmt.Errorf("initializing synthetic-only CV processor: %w", err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	staging, err := newStaging(ctx, *baseMigrations, *adminFixtures, *integrationFixtures, processor)
	if err != nil {
		return fmt.Errorf("menyiapkan staging sintetis: %w", err)
	}
	defer staging.close()

	server := httpapi.Server{
		Config: httpapi.ServerConfig{
			Version:        "FL-ENR02A-synthetic",
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   20 * time.Second,
			IdleTimeout:    60 * time.Second,
			MaxBodyBytes:   httpapi.DefaultMaxBodyBytes,
			RequestTimeout: httpapi.DefaultRequestTimeout,
		},
		DB:         staging.db,
		Migrations: staging.migration,
		Logger:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Accounts:   staging.accounts,
		Sessions:   staging.sessions,
		EmailOTP:   staging.otp,
		Audit:      audit.NewWriter(staging.db, staging.clk),
		Scope:      authz.NewSQLDirectory(staging.db),
		Clock:      staging.clk,
		Enrollment: staging.service,
	}
	handler, err := stagingWebHandler(httpapi.NewRouter(server), *webRoot)
	if err != nil {
		return err
	}

	rawListener, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("mendengarkan: %w", err)
	}
	listener := rawListener
	scheme := "https"
	if *insecureHTTP {
		scheme = "http"
	} else {
		pair, err := tls.LoadX509KeyPair(*cert, *key)
		if err != nil {
			_ = rawListener.Close()
			return fmt.Errorf("memuat sertifikat dev: %w", err)
		}
		listener = tls.NewListener(rawListener, &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12})
	}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() {
		err := srv.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()

	// The disposable harness has a bounded worker and periodic reconciliation.
	// It does not exist in cmd/server.
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		process := time.NewTicker(400 * time.Millisecond)
		reconcile := time.NewTicker(10 * time.Second)
		defer process.Stop()
		defer reconcile.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-process.C:
				if staging.control == nil {
					_ = staging.service.ProcessNext(ctx)
				} else {
					staging.control.workMu.Lock()
					if !staging.control.workerPaused() {
						_ = staging.service.ProcessNext(ctx)
					}
					staging.control.workMu.Unlock()
				}
			case <-reconcile.C:
				if staging.control != nil {
					staging.control.workMu.Lock()
				}
				_ = staging.service.Reconcile(ctx)
				if staging.control != nil {
					staging.control.workMu.Unlock()
				}
			}
		}
	}()
	defer func() {
		stop()
		<-workerDone // Reap an in-flight Python child before closing/removing its disposable state.
		if staging.control != nil {
			// stdin worker.step uses the same slot as the periodic worker.
			staging.control.workMu.Lock()
			staging.control.workMu.Unlock()
		}
	}()

	baseURL := fmt.Sprintf("%s://%s", scheme, listener.Addr().String())
	startup := map[string]any{
		"mode":          "FL-ENR02A-synthetic-staging",
		"base_url":      baseURL,
		"schema":        staging.migration.CurrentVersion,
		"student_email": staging.student.Email,
		"admin_email":   staging.admin.Email,
		"person_id":     staging.person.ID,
		"class_id":      staging.class.ID,
		"otp_file":      staging.otpPath,
		"root":          staging.root,
		"production":    false,
		"human_capture": false,
	}
	if *testTokens {
		studentPair, err := staging.sessions.Issue(ctx, staging.student.ID, "enr02a-e2e-student")
		if err != nil {
			_ = srv.Close()
			return fmt.Errorf("menerbitkan sesi fixture mahasiswa: %w", err)
		}
		adminPair, err := staging.sessions.Issue(ctx, staging.admin.ID, "enr02a-e2e-admin")
		if err != nil {
			_ = srv.Close()
			return fmt.Errorf("menerbitkan sesi fixture admin: %w", err)
		}
		startup["student_token"] = studentPair.AccessToken
		startup["admin_token"] = adminPair.AccessToken
	}
	if *adminFixtures {
		startup["mode"] = "FL-ENR03-synthetic-staging"
		startup["fixtures"] = staging.fixtures
		startup["worker_paused"] = true
		startup["fixture_control"] = "stdin-jsonl"
		startup["server_now"] = staging.clk.Now().UTC().Format(time.RFC3339Nano)
	}
	if processor != nil {
		startup["mode"] = "FL-ENR04-synthetic-real-model-staging"
		startup["processor"] = processor.Manifest()
	}
	if *integrationFixtures {
		startup["mode"] = "FL-ENR05-synthetic-integration"
		startup["trace_id"] = staging.integration.traceID
		startup["fixtures"] = staging.fixtures
		startup["worker_paused"] = true
		startup["fixture_control"] = "stdin-jsonl"
		startup["server_now"] = staging.clk.Now().UTC().Format(time.RFC3339Nano)
	}
	if err = json.NewEncoder(os.Stdout).Encode(startup); err != nil {
		_ = srv.Close()
		return fmt.Errorf("menulis startup JSON: %w", err)
	}
	if *adminFixtures {
		go staging.readControls(ctx, os.Stdin, os.Stdout, stop)
	} else if *integrationFixtures {
		go staging.readIntegrationControls(ctx, os.Stdin, os.Stdout, stop)
	} else {
		go staging.readOTPControls(ctx, os.Stdin, os.Stdout, stop)
	}

	select {
	case <-ctx.Done():
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("server staging berhenti: %w", err)
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}
