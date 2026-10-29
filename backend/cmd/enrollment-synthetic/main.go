// Command enrollment-synthetic exercises the real enrollment router over TLS.
// It accepts no database/key/image paths and only creates disposable synthetic data.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/authz"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentfixture"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/httpapi"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/session"
	"github.com/google/uuid"
)

// This processor is deliberately confined to a synthetic verification command.
// It is not SFace, does not detect identity, and cannot be enabled in cmd/server.
type syntheticProcessor struct{}

func (syntheticProcessor) Manifest() enrollment.Manifest {
	h := sha256.Sum256([]byte("FL-ENR01B-SYNTHETIC-NOT-A-MODEL"))
	d := hex.EncodeToString(h[:])
	return enrollment.Manifest{SyntheticOnly: true, ModelID: "synthetic-sface", ModelVersion: "test-v1", ModelChecksum: d, DetectorID: "synthetic-yunet", DetectorVersion: "test-v1", DetectorChecksum: d, PipelineRevision: "synthetic-v1"}
}
func (p syntheticProcessor) Process(ctx context.Context, b []byte) (enrollment.ProcessingResult, error) {
	if e := ctx.Err(); e != nil {
		return enrollment.ProcessingResult{}, e
	}
	if _, e := jpeg.Decode(bytes.NewReader(b)); e != nil {
		return enrollment.ProcessingResult{}, e
	}
	v := make([]float32, 128)
	v[0] = 1
	return enrollment.ProcessingResult{Manifest: p.Manifest(), FaceCount: 1, Embedding: v}, nil
}

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "FL-ENR01B synthetic verification FAILED:", e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 1 {
		return fmt.Errorf("this synthetic command accepts no arguments")
	}
	ctx := context.Background()
	root, e := os.MkdirTemp("", "facelink-enr01b-synthetic-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(root)
	db, e := database.Open(ctx, filepath.Join(root, "synthetic.db"))
	if e != nil {
		return e
	}
	defer db.Close()
	baseDir, e := filepath.Abs("migrations")
	if e != nil {
		return e
	}
	migrationDir, e := enrollmentfixture.PrepareMigrations(baseDir, filepath.Join(root, "migrations"))
	if e != nil {
		return e
	}
	migration, e := migrations.Run(ctx, db, migrationDir)
	if e != nil {
		return e
	}
	clk := clock.NewFake(time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC))
	accounts := auth.NewStore(db, clk)
	admin, e := accounts.CreateFirstAdmin(ctx, "admin.synthetic@campus.example", "Admin Sintetis")
	if e != nil {
		return e
	}
	// Fixture activation only, on this newly created disposable DB; never OTP bypass in product.
	if _, e = db.ExecContext(ctx, "UPDATE users SET status='active' WHERE id=?", admin.ID); e != nil {
		return e
	}
	student, e := accounts.InviteUser(ctx, "student.synthetic@campus.example", "Mahasiswa Sintetis", auth.RoleMahasiswa, admin.ID)
	if e != nil {
		return e
	}
	lecturer, e := accounts.InviteUser(ctx, "lecturer.synthetic@campus.example", "Dosen Sintetis", auth.RoleDosen, admin.ID)
	if e != nil {
		return e
	}
	if _, e = db.ExecContext(ctx, "UPDATE users SET status='active' WHERE id IN (?,?)", student.ID, lecturer.ID); e != nil {
		return e
	}
	person, e := accounts.CreatePerson(ctx, admin.ID, "SYN-ENR01B", "Mahasiswa Sintetis", "", "roster-synthetic-v1")
	if e != nil {
		return e
	}
	class, e := accounts.CreateClass(ctx, admin.ID, "SYN-ENR01B", "Kelas Sintetis")
	if e != nil {
		return e
	}
	if _, e = accounts.AddClassMember(ctx, admin.ID, class.ID, person.ID); e != nil {
		return e
	}
	if _, e = accounts.LinkPersonAccount(ctx, admin.ID, person.ID, student.ID); e != nil {
		return e
	}
	keyDir := filepath.Join(root, "keys")
	if e = enrollmentsecure.Provision(keyDir); e != nil {
		return e
	}
	keys, e := enrollmentsecure.LoadKeyProvider(keyDir)
	if e != nil {
		return e
	}
	files, e := enrollmentsecure.NewFileStore(filepath.Join(root, "photos"))
	if e != nil {
		return e
	}
	svc, e := enrollment.New(db, clk, keys, files, syntheticProcessor{})
	if e != nil {
		return e
	}
	if e = svc.Reconcile(ctx); e != nil {
		return e
	}
	sessions := session.NewStore(db, clk, session.Config{})
	ap, e := sessions.Issue(ctx, admin.ID, "synthetic")
	if e != nil {
		return e
	}
	sp, e := sessions.Issue(ctx, student.ID, "synthetic")
	if e != nil {
		return e
	}
	lp, e := sessions.Issue(ctx, lecturer.ID, "synthetic")
	if e != nil {
		return e
	}
	server := httpapi.Server{DB: db, Migrations: migration, Accounts: accounts, Sessions: sessions, Scope: authz.NewSQLDirectory(db), Audit: audit.NewWriter(db, clk), Clock: clk, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Enrollment: svc}
	tls := httptest.NewTLSServer(httpapi.NewRouter(server))
	defer tls.Close()
	type reply struct {
		Data  map[string]any `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	statuses := map[string]int{}
	send := func(name, method, path, token string, body []byte, headers map[string]string, want int) (reply, http.Header, error) {
		req, er := http.NewRequest(method, tls.URL+path, bytes.NewReader(body))
		if er != nil {
			return reply{}, nil, er
		}
		req.Header.Set("Authorization", "Bearer "+token)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, er := tls.Client().Do(req)
		if er != nil {
			return reply{}, nil, er
		}
		defer res.Body.Close()
		b, er := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		if er != nil {
			return reply{}, nil, er
		}
		defer clear(b)
		statuses[name] = res.StatusCode
		if res.TLS == nil || res.StatusCode != want {
			return reply{}, nil, fmt.Errorf("%s HTTP %d expected %d", name, res.StatusCode, want)
		}
		var out reply
		if res.Header.Get("Content-Type") == "image/jpeg" {
			if _, er = jpeg.Decode(bytes.NewReader(b)); er != nil {
				return out, nil, er
			}
		} else if er = json.Unmarshal(b, &out); er != nil {
			return out, nil, er
		}
		return out, res.Header, nil
	}
	jsonBody := func(v any) []byte { b, _ := json.Marshal(v); return b }
	jheaders := func(tag string) map[string]string {
		h := map[string]string{"Content-Type": "application/json", "Idempotency-Key": uuid.NewString()}
		if tag != "" {
			h["If-Match"] = tag
		}
		return h
	}
	_, noticeHeaders, e := send("notice", "GET", "/api/v1/me/consent", sp.AccessToken, nil, nil, 200)
	if e != nil {
		return e
	}
	granted, _, e := send("grant", "POST", "/api/v1/me/consent", sp.AccessToken, jsonBody(map[string]any{"accepted": true, "notice_version": enrollment.NoticeVersion}), jheaders(noticeHeaders.Get("ETag")), 201)
	if e != nil {
		return e
	}
	consent := granted.Data["resource_id"].(string)
	im := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := range 128 {
		for x := range 128 {
			im.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 100, 255})
		}
	}
	var jpg bytes.Buffer
	if e = jpeg.Encode(&jpg, im, &jpeg.Options{Quality: 85}); e != nil {
		return e
	}
	defer clear(jpg.Bytes())
	headers := map[string]string{"Content-Type": "image/jpeg", "X-Consent-Id": consent, "Idempotency-Key": uuid.NewString()}
	submitted, _, e := send("submit", "POST", "/api/v1/me/face-enrollment-submissions", sp.AccessToken, jpg.Bytes(), headers, 201)
	if e != nil {
		return e
	}
	id := submitted.Data["resource_id"].(string)
	replay, _, e := send("replay", "POST", "/api/v1/me/face-enrollment-submissions", sp.AccessToken, jpg.Bytes(), headers, 200)
	if e != nil {
		return e
	}
	if replay.Data["resource_id"] != id {
		return fmt.Errorf("replay changed identity")
	}
	path := "/api/v1/face-enrollment-submissions/" + id
	if _, _, e = send("dosen_preview_denied", "GET", path+"/photo", lp.AccessToken, nil, nil, 403); e != nil {
		return e
	}
	if _, _, e = send("student_preview_denied", "GET", path+"/photo", sp.AccessToken, nil, nil, 403); e != nil {
		return e
	}
	_, detailHeaders, e := send("admin_detail", "GET", path, ap.AccessToken, nil, nil, 200)
	if e != nil {
		return e
	}
	if _, _, e = send("admin_private_preview", "GET", path+"/photo", ap.AccessToken, nil, nil, 200); e != nil {
		return e
	}
	approved, _, e := send("approve", "POST", path+"/approve", ap.AccessToken, jsonBody(map[string]any{"confirmed": true, "identity_check_method": "CAMPUS_AUTHORIZED_CHECK", "attestation_version": "synthetic-v1", "campus_check_ref": uuid.NewString()}), jheaders(detailHeaders.Get("ETag")), 202)
	if e != nil {
		return e
	}
	if approved.Data["current"].(map[string]any)["effective_status"] != "APPROVED_PROCESSING" {
		return fmt.Errorf("approval activated too early")
	}
	if e = svc.ProcessNext(ctx); e != nil {
		return e
	}
	active, _, e := send("active_synthetic", "GET", "/api/v1/me/face-enrollment", sp.AccessToken, nil, nil, 200)
	if e != nil {
		return e
	}
	if active.Data["effective_status"] != "ACTIVE" || active.Data["template_ready"] != true {
		return fmt.Errorf("synthetic activation failed")
	}
	// Fault injection only in this disposable key directory. No replacement key.
	templateKey := filepath.Join(keyDir, enrollmentsecure.Template+"-1.key")
	if e = os.Rename(templateKey, templateKey+".unavailable"); e != nil {
		return e
	}
	unavailable, _, e := send("key_loss_fail_closed", "GET", "/api/v1/me/face-enrollment", sp.AccessToken, nil, nil, 200)
	if e != nil {
		return e
	}
	if unavailable.Data["template_ready"] != false {
		return fmt.Errorf("missing key did not close readiness")
	}
	var retained int
	if e = db.QueryRowContext(ctx, "SELECT count(*) FROM face_templates").Scan(&retained); e != nil || retained != 1 {
		return fmt.Errorf("key loss deleted template")
	}
	if _, e = os.Stat(templateKey); !os.IsNotExist(e) {
		return fmt.Errorf("key was silently replaced")
	}
	attendance, _, e := send("manual_session_create", "POST", "/api/v1/sessions", ap.AccessToken, jsonBody(map[string]any{"class_id": class.ID, "title": "Manual tanpa kunci biometrik", "closes_at": clk.Now().Add(time.Hour).Format(time.RFC3339Nano)}), jheaders(""), 201)
	if e != nil {
		return e
	}
	attendanceID, ok := attendance.Data["id"].(string)
	if !ok {
		return fmt.Errorf("manual session response missing ID")
	}
	if _, _, e = send("manual_session_open", "POST", "/api/v1/sessions/"+attendanceID+"/open", ap.AccessToken, jsonBody(map[string]any{}), jheaders(""), 200); e != nil {
		return e
	}
	manual, _, e := send("manual_without_key", "POST", "/api/v1/sessions/"+attendanceID+"/manual-checkins", ap.AccessToken, jsonBody(map[string]any{"person_id": person.ID, "status": "present", "reason": "Fixture sintetis tanpa kunci biometrik"}), jheaders(""), 201)
	if e != nil {
		return e
	}
	if manual.Data["status"] != "present" || manual.Data["source"] != "manual" {
		return fmt.Errorf("manual fallback did not record valid attendance")
	}
	if _, _, e = send("withdraw", "POST", "/api/v1/me/consent/withdraw", sp.AccessToken, jsonBody(map[string]any{"scope": "ALL_FACE_ENROLLMENT", "confirmed": true}), jheaders(""), 200); e != nil {
		return e
	}
	if e = svc.Reconcile(ctx); e != nil {
		return e
	}
	final, _, e := send("revoked", "GET", "/api/v1/me/face-enrollment", sp.AccessToken, nil, nil, 200)
	if e != nil {
		return e
	}
	if final.Data["template_ready"] != false || final.Data["effective_status"] != "REVOKED" {
		return fmt.Errorf("withdraw did not revoke")
	}
	var templates int
	if e = db.QueryRowContext(ctx, "SELECT count(*) FROM face_templates").Scan(&templates); e != nil {
		return e
	}
	if templates != 0 {
		return fmt.Errorf("template survived withdrawal")
	}
	ids, e := files.IDs()
	if e != nil {
		return e
	}
	if len(ids) != 0 {
		return fmt.Errorf("ciphertext survived reconciliation")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"task": "FL-ENR01B", "mode": "synthetic_only", "https": true, "schema": migration.CurrentVersion, "statuses": statuses, "templateRowsAfterWithdrawal": templates, "encryptedFilesAfterReconciliation": len(ids), "keyProvider": "explicit isolated DPAPI CurrentUser on Windows", "realSFaceTested": false, "productionEnabled": false, "disposableCleanupOnExit": true})
}
