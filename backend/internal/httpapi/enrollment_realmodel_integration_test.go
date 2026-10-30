package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentcv"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollmentsecure"
	"github.com/google/uuid"
)

// TestEnrollmentRealModelSyntheticLifecycle is deliberately separate from the
// fake-processor domain/HTTP tests. Setting both fixture variables attests that
// the file is wholly procedurally generated, not a human image or old biometric.
// No fixture, interpreter, engine or model path is discovered by default.
func TestEnrollmentRealModelSyntheticLifecycle(t *testing.T) {
	started := time.Now()
	ctx := context.Background()
	jpg, processor := realModelSyntheticConfig(t)
	defer clear(jpg)
	fixtureHash := sha256.Sum256(jpg)
	manifest := processor.Manifest()
	h := newHarnessAuth(t) // Existing local-only synthetic JWKS; no SMTP or OTP.
	enrApplySchema(t, h)
	var journal string
	if err := h.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil || !strings.EqualFold(journal, "wal") {
		t.Fatal("disposable integration database must use WAL")
	}

	// Only this test owns these random keys; no runtime key-extraction hook.
	keys := enrHTTPKeys{}
	for _, purpose := range []string{enrollmentsecure.PendingPhoto, enrollmentsecure.Template} {
		keys[purpose] = make([]byte, 32)
		if _, err := rand.Read(keys[purpose]); err != nil {
			t.Fatal("could not generate test-only purpose key")
		}
		defer clear(keys[purpose])
	}
	if bytes.Equal(keys[enrollmentsecure.PendingPhoto], keys[enrollmentsecure.Template]) {
		t.Fatal("purpose keys must differ")
	}
	files, err := enrollmentsecure.NewFileStore(filepath.Join(t.TempDir(), "ciphertext"))
	if err != nil {
		t.Fatal("could not create disposable ciphertext store")
	}
	svc, err := enrollment.New(h.db, h.clk, keys, files, processor)
	if err != nil {
		t.Fatal("could not construct real-model enrollment service")
	}
	h.srv.Enrollment = svc
	h.router = NewRouter(h.srv)
	https := realModelHTTPS(t, h.router)
	h.router = https

	_, admin := h.adminPertama("admin.realmodel@kampus.example")
	studentID, student := h.undangDanMasuk(admin.Access, "student.realmodel@kampus.example", "Mahasiswa Sintetis", "mahasiswa")
	person := corePerson(h, admin.Access, "ENR-REAL-SYN-001")
	class := coreClass(h, admin.Access, "ENR-REAL-SYN")
	realModelData(t, h.kirim(http.MethodPut, "/api/v1/classes/"+class+"/members/"+person, nil, admin.Access), http.StatusCreated)
	realModelData(t, h.kirim(http.MethodPut, "/api/v1/people/"+person+"/account-link", map[string]any{"user_id": studentID}, admin.Access), http.StatusOK)

	notice := h.kirim(http.MethodGet, "/api/v1/me/consent", nil, student.Access)
	realModelData(t, notice, http.StatusOK)
	grant := realModelData(t, enrJSON(h, "/api/v1/me/consent", student.Access, map[string]any{"notice_version": enrollment.NoticeVersion, "accepted": true}, notice.Rec.Header().Get("ETag"), uuid.NewString()), http.StatusCreated)
	consent := realModelID(t, grant)
	submitted := realModelData(t, enrSubmit(h, student.Access, consent, uuid.NewString(), jpg), http.StatusCreated)
	submission := realModelID(t, submitted)
	path := "/api/v1/face-enrollment-submissions/" + submission
	if h.hitung("SELECT COUNT(*) FROM face_templates") != 0 || h.hitung("SELECT COUNT(*) FROM enrollment_processing_jobs") != 0 {
		t.Fatal("upload must not extract or activate")
	}

	canonical, width, height, err := enrollmentsecure.NormalizeJPEG(jpg)
	if err != nil {
		t.Fatal("pinned procedural fixture is not an accepted enrollment JPEG")
	}
	defer clear(canonical)
	canonicalHash := sha256.Sum256(canonical)
	var storedHash string
	if err = h.db.QueryRowContext(ctx, "SELECT canonical_sha256 FROM face_enrollment_submissions WHERE id=?", submission).Scan(&storedHash); err != nil || storedHash != hex.EncodeToString(canonicalHash[:]) {
		t.Fatal("submission did not bind the canonical image hash")
	}
	photo, err := files.Read(submission)
	if err != nil {
		t.Fatal("pending encrypted photo is not readable")
	}
	photoBinding := enrollmentsecure.Binding{Purpose: enrollmentsecure.PendingPhoto, RecordID: submission, PersonID: person, UserID: studentID, ConsentID: consent}
	realModelEnvelope(t, photo, 1, len(canonical))
	openedPhoto, err := enrollmentsecure.Open(ctx, keys, photoBinding, photo)
	if err != nil {
		t.Fatal("authorized pending-photo decryption failed")
	}
	if !bytes.Equal(openedPhoto, canonical) || bytes.Equal(photo.Ciphertext, canonical) {
		clear(openedPhoto)
		t.Fatal("pending-photo encryption/canonicalization invariant failed")
	}
	clear(openedPhoto)
	var photoNonce []byte
	var photoVersion, photoState string
	if err = h.db.QueryRowContext(ctx, "SELECT nonce,key_version,storage_state FROM face_enrollment_photos WHERE submission_id=?", submission).Scan(&photoNonce, &photoVersion, &photoState); err != nil || !bytes.Equal(photoNonce, photo.Nonce) || photoVersion != photo.KeyVersion || photoState != "READY" {
		t.Fatal("pending-photo database/file envelope mismatch")
	}
	wrongPhoto := photoBinding
	wrongPhoto.ConsentID = uuid.NewString()
	realModelMustReject(t, keys, wrongPhoto, photo)
	wrongPhoto = photoBinding
	wrongPhoto.RecordID = uuid.NewString()
	realModelMustReject(t, keys, wrongPhoto, photo)
	wrongPhoto = photoBinding
	wrongPhoto.UserID = uuid.NewString()
	realModelMustReject(t, keys, wrongPhoto, photo)
	wrongKeys := enrHTTPKeys{enrollmentsecure.PendingPhoto: keys[enrollmentsecure.Template], enrollmentsecure.Template: keys[enrollmentsecure.PendingPhoto]}
	realModelMustReject(t, wrongKeys, photoBinding, photo)

	detail := h.kirim(http.MethodGet, path, nil, admin.Access)
	pending := realModelData(t, detail, http.StatusOK)
	if pending["effective_status"] != "PENDING_APPROVAL" || pending["template_ready"] != false {
		t.Fatal("uploaded submission must await approval without a template")
	}
	preview := enrSend(h, http.MethodGet, path+"/photo", admin.Access, nil, nil)
	if preview.Status != http.StatusOK || preview.Rec.Header().Get("Cache-Control") != "no-store" || !bytes.Equal(preview.Rec.Body.Bytes(), canonical) {
		t.Fatal("authorized review did not receive exactly the canonical photo")
	}
	clear(preview.Rec.Body.Bytes())
	approval := realModelData(t, enrJSON(h, path+"/approve", admin.Access, map[string]any{"identity_check_method": "CAMPUS_AUTHORIZED_CHECK", "attestation_version": "synthetic-v1", "confirmed": true, "campus_check_ref": uuid.NewString()}, detail.Rec.Header().Get("ETag"), uuid.NewString()), http.StatusAccepted)
	current, ok := approval["current"].(map[string]any)
	if !ok || current["effective_status"] != "APPROVED_PROCESSING" || current["template_ready"] != false {
		t.Fatal("approval202 must queue, not activate")
	}
	var expectedHash, jobState string
	if err = h.db.QueryRowContext(ctx, "SELECT expected_photo_hash,state FROM enrollment_processing_jobs WHERE submission_id=?", submission).Scan(&expectedHash, &jobState); err != nil || expectedHash != storedHash || jobState != "QUEUED" {
		t.Fatal("approval did not queue work bound to the reviewed canonical bytes")
	}

	processingStarted := time.Now()
	if err = svc.ProcessNext(ctx); err != nil {
		t.Fatalf("real-model ProcessNext failed: %v", err)
	}
	processingElapsed := time.Since(processingStarted)
	active := realModelData(t, h.kirim(http.MethodGet, "/api/v1/me/face-enrollment", nil, student.Access), http.StatusOK)
	if active["effective_status"] != "ACTIVE" || active["template_ready"] != true || active["synthetic_only"] != true {
		// Failure is terminal for this test: never replace the real processor.
		var reason string
		if err = h.db.QueryRowContext(ctx, "SELECT COALESCE(last_error_code,''),state FROM enrollment_processing_jobs WHERE submission_id=?", submission).Scan(&reason, &jobState); err != nil {
			t.Fatal("real-model activation failed and job outcome is unavailable")
		}
		templateRows := h.hitung("SELECT COUNT(*) FROM face_templates")
		activeRows := h.hitung("SELECT COUNT(*) FROM face_enrollments WHERE status='active'")
		photoRows := h.hitung("SELECT COUNT(*) FROM face_enrollment_photos")
		if err = svc.Reconcile(ctx); err != nil {
			t.Fatal("rejected-fixture reconciliation failed")
		}
		fileIDs, fileErr := files.IDs()
		if fileErr != nil {
			t.Fatal("cannot inspect rejected-fixture cleanup")
		}
		t.Logf("synthetic_only=true fixture_sha256=%x canonical_sha256=%x dimensions=%dx%d processing_ms=%d total_ms=%d job_state=%s reason_code=%s template_rows=%d active_rows=%d photo_rows=%d ciphertext_files=%d", fixtureHash, canonicalHash, width, height, processingElapsed.Milliseconds(), time.Since(started).Milliseconds(), jobState, reason, templateRows, activeRows, photoRows, len(fileIDs))
		t.Logf("model_id=%s model_version=%s model_sha256=%s detector_id=%s detector_version=%s detector_sha256=%s pipeline_revision=%s", manifest.ModelID, manifest.ModelVersion, manifest.ModelChecksum, manifest.DetectorID, manifest.DetectorVersion, manifest.DetectorChecksum, manifest.PipelineRevision)
		if active["template_ready"] != false || templateRows != 0 || activeRows != 0 {
			t.Fatal("rejected-fixture path left an active enrollment or template")
		}
		t.Fatalf("pinned procedural fixture did not activate with real models: reason_code=%s", reason)
	}

	var template enrollmentsecure.Envelope
	var stored enrollment.Manifest
	var dimension int
	var dtype, templateID, templatePerson, templateConsent, templateSubmission string
	err = h.db.QueryRowContext(ctx, `SELECT id,envelope_version,algorithm,key_version,aad,nonce,ciphertext,person_id,consent_id,submission_id,embedding_dim,dtype,model_id,model_version,model_checksum,detector_id,detector_version,detector_checksum,pipeline_revision,synthetic_only FROM face_templates WHERE submission_id=?`, submission).Scan(&templateID, &template.Version, &template.Algorithm, &template.KeyVersion, &template.AAD, &template.Nonce, &template.Ciphertext, &templatePerson, &templateConsent, &templateSubmission, &dimension, &dtype, &stored.ModelID, &stored.ModelVersion, &stored.ModelChecksum, &stored.DetectorID, &stored.DetectorVersion, &stored.DetectorChecksum, &stored.PipelineRevision, &stored.SyntheticOnly)
	if err != nil || stored != manifest || dimension != 128 || dtype != "float32-le" || templatePerson != person || templateConsent != consent || templateSubmission != submission || uuid.Validate(templateID) != nil || h.hitung("SELECT COUNT(*) FROM face_templates") != 1 {
		t.Fatal("activated template metadata/fingerprint/binding mismatch")
	}
	realModelEnvelope(t, template, enrollmentsecure.TemplateVersion, 512)
	if bytes.Equal(template.Nonce, photo.Nonce) {
		t.Fatal("photo and template reused an AES-GCM nonce")
	}
	templateBinding := enrollmentsecure.Binding{Purpose: enrollmentsecure.Template, RecordID: templateID, SubmissionID: submission, PersonID: person, ConsentID: consent, ModelID: stored.ModelID, ModelVersion: stored.ModelVersion, ModelFingerprint: stored.Fingerprint()}
	plain, err := enrollmentsecure.Open(ctx, keys, templateBinding, template)
	if err != nil {
		t.Fatal("authorized template decryption failed")
	}
	defer clear(plain)
	if len(plain) != 512 || bytes.Contains(template.Ciphertext, plain) {
		t.Fatal("template plaintext/ciphertext length invariant failed")
	}
	var norm float64
	for i := range 128 {
		value := float64(math.Float32frombits(binary.LittleEndian.Uint32(plain[i*4 : i*4+4])))
		if math.IsNaN(value) || math.IsInf(value, 0) {
			t.Fatal("decrypted real-model template contains a nonfinite component")
		}
		norm += value * value
	}
	if math.Abs(math.Sqrt(norm)-1) > 1e-5 {
		t.Fatal("decrypted real-model template is not unit-normalized")
	}
	clear(plain)
	for _, change := range []func(*enrollmentsecure.Binding){
		func(b *enrollmentsecure.Binding) { b.RecordID = uuid.NewString() },
		func(b *enrollmentsecure.Binding) { b.SubmissionID = uuid.NewString() },
		func(b *enrollmentsecure.Binding) { b.PersonID = uuid.NewString() },
		func(b *enrollmentsecure.Binding) { b.ConsentID = uuid.NewString() },
		func(b *enrollmentsecure.Binding) { b.ModelID = "wrong-model" },
		func(b *enrollmentsecure.Binding) { b.ModelVersion = "wrong-version" },
		func(b *enrollmentsecure.Binding) { b.ModelFingerprint = strings.Repeat("0", 64) },
	} {
		wrong := templateBinding
		change(&wrong)
		realModelMustReject(t, keys, wrong, template)
	}
	realModelMustReject(t, keys, photoBinding, template)
	realModelMustReject(t, wrongKeys, templateBinding, template)
	tampered := template
	tampered.Ciphertext = bytes.Clone(template.Ciphertext)
	tampered.Ciphertext[0] ^= 1
	realModelMustReject(t, keys, templateBinding, tampered)
	tampered = template
	tampered.Nonce = bytes.Clone(template.Nonce)
	tampered.Nonce[0] ^= 1
	realModelMustReject(t, keys, templateBinding, tampered)
	tampered = template
	tampered.AAD += "00"
	realModelMustReject(t, keys, templateBinding, tampered)
	if h.hitung("SELECT COUNT(*) FROM face_enrollment_photos") != 0 || h.hitung("SELECT COUNT(*) FROM face_enrollments WHERE status='active'") != 1 || h.hitung("SELECT COUNT(*) FROM enrollment_processing_jobs WHERE state='FINISHED' AND last_error_code IS NULL") != 1 {
		t.Fatal("atomic activation/job/photo purge invariant failed")
	}
	if err = svc.Reconcile(ctx); err != nil {
		t.Fatal("post-activation reconciliation failed")
	}
	realModelNoFiles(t, files)

	const historicalGrant = `SELECT json_array(id,person_id,subject_user_id,granted_at,notice_version,notice_digest,policy_version) FROM consent_records WHERE id=? AND person_id=? AND subject_user_id=?`
	var grantBefore, grantAfter, grantStatus string
	if err = h.db.QueryRowContext(ctx, historicalGrant, consent, person, studentID).Scan(&grantBefore); err != nil {
		t.Fatal("historical consent grant unavailable before withdrawal")
	}
	realModelData(t, enrJSON(h, "/api/v1/me/consent/withdraw", student.Access, map[string]any{"scope": "ALL_FACE_ENROLLMENT", "confirmed": true}, "", uuid.NewString()), http.StatusOK)
	if err = svc.Reconcile(ctx); err != nil {
		t.Fatal("post-withdrawal reconciliation failed")
	}
	withdrawn := realModelData(t, h.kirim(http.MethodGet, "/api/v1/me/face-enrollment", nil, student.Access), http.StatusOK)
	if withdrawn["effective_status"] != "REVOKED" || withdrawn["template_ready"] != false || withdrawn["deletion_state"] != "COMPLETE" || h.hitung("SELECT COUNT(*) FROM face_templates") != 0 || h.hitung("SELECT COUNT(*) FROM face_enrollment_photos") != 0 || h.hitung("SELECT COUNT(*) FROM face_enrollments WHERE status='active'") != 0 || h.hitung("SELECT COUNT(*) FROM biometric_deletion_jobs WHERE state<>'COMPLETE'") != 0 {
		t.Fatal("withdrawal left ready enrollment, payload or incomplete deletion")
	}
	if err = h.db.QueryRowContext(ctx, historicalGrant, consent, person, studentID).Scan(&grantAfter); err != nil || grantAfter != grantBefore {
		t.Fatal("withdrawal changed the immutable consent grant identity/evidence")
	}
	if err = h.db.QueryRowContext(ctx, `SELECT status FROM consent_records WHERE id=?`, consent).Scan(&grantStatus); err != nil || grantStatus != "withdrawn" {
		t.Fatal("withdrawal did not update consent status")
	}
	// Crypto-only retained test envelope: authorization has been revoked and the
	// actual stored template deleted. Mutable withdrawal state must not rewrite
	// the historical grant identity authenticated by AAD.
	historicalPlain, historicalErr := enrollmentsecure.Open(ctx, keys, templateBinding, template)
	historicalValid := historicalErr == nil && len(historicalPlain) == 512
	clear(historicalPlain)
	if !historicalValid {
		t.Fatal("withdrawal unintentionally invalidated historical AAD authentication")
	}
	realModelNoFiles(t, files)
	realModelAudit(t, h, admin.UserID, studentID, submission)
	logs := h.logs.String()
	for _, secret := range []string{admin.Access, admin.Refresh, student.Access, student.Refresh, string(jpg), string(canonical), hex.EncodeToString(keys[enrollmentsecure.PendingPhoto]), hex.EncodeToString(keys[enrollmentsecure.Template]), "ciphertext", "embedding", "campus_check_ref", "identity_check_method"} {
		if secret != "" && strings.Contains(logs, secret) {
			t.Fatal("application log contains protected enrollment/auth material")
		}
	}
	if https.requests == 0 {
		t.Fatal("integration did not complete any verified HTTPS requests")
	}
	t.Logf("https_verified=true explicit_test_ca=true loopback_only=true https_requests=%d protected_material_logged=false", https.requests)
	t.Logf("synthetic_only=true fixture_sha256=%x canonical_sha256=%x dimensions=%dx%d embedding_dim=128 plaintext_bytes=512 ciphertext_bytes=%d algorithm=%s processing_ms=%d total_ms=%d terminal=REVOKED template_ready=false payload_rows=0 ciphertext_files=0", fixtureHash, canonicalHash, width, height, len(template.Ciphertext), template.Algorithm, processingElapsed.Milliseconds(), time.Since(started).Milliseconds())
	t.Logf("model_id=%s model_version=%s model_sha256=%s detector_id=%s detector_version=%s detector_sha256=%s pipeline_revision=%s", manifest.ModelID, manifest.ModelVersion, manifest.ModelChecksum, manifest.DetectorID, manifest.DetectorVersion, manifest.DetectorChecksum, manifest.PipelineRevision)
	t.Logf("template_envelope_version=%d immutable_grant_preserved=true record_submission_consent_fingerprint_binding_verified=true", template.Version)
}

// The adapter preserves the existing local auth/request helpers, but every call
// crosses a real loopback TLS socket before reaching the production router.
type realModelHTTPSBridge struct {
	t        *testing.T
	client   *http.Client
	endpoint *url.URL
	cert     *x509.Certificate
	requests int
}

func realModelHTTPS(t *testing.T, router http.Handler) *realModelHTTPSBridge {
	t.Helper()
	server := httptest.NewTLSServer(router)
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL)
	if err != nil || endpoint.Scheme != "https" || !net.ParseIP(endpoint.Hostname()).IsLoopback() {
		t.Fatal("integration HTTPS server must bind a loopback address")
	}
	cert := server.Certificate()
	if cert == nil || cert.VerifyHostname(endpoint.Hostname()) != nil {
		t.Fatal("integration HTTPS certificate must identify its loopback host")
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
	}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &realModelHTTPSBridge{t: t, client: client, endpoint: endpoint, cert: cert}
}

func (b *realModelHTTPSBridge) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	b.t.Helper()
	if request.URL.IsAbs() || request.URL.Host != "" {
		b.t.Fatal("integration request must remain on the disposable loopback server")
	}
	outbound := request.Clone(request.Context())
	outbound.URL.Scheme = b.endpoint.Scheme
	outbound.URL.Host = b.endpoint.Host
	outbound.Host = b.endpoint.Host
	outbound.RequestURI = ""
	outbound.TLS = nil
	response, err := b.client.Do(outbound)
	if err != nil {
		b.t.Fatal("disposable HTTPS request failed (request and payload omitted)")
	}
	defer response.Body.Close()
	if response.TLS == nil || len(response.TLS.VerifiedChains) == 0 || len(response.TLS.PeerCertificates) == 0 || !response.TLS.PeerCertificates[0].Equal(b.cert) {
		b.t.Fatal("HTTPS response did not verify against the explicit disposable certificate")
	}
	b.requests++
	for name, values := range response.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	if _, err = io.Copy(w, response.Body); err != nil {
		b.t.Fatal("could not read disposable HTTPS response (payload omitted)")
	}
}

func realModelSyntheticConfig(t *testing.T) ([]byte, *enrollmentcv.Processor) {
	t.Helper()
	names := []string{"FACELINK_CV_SYNTHETIC_JPEG", "FACELINK_CV_SYNTHETIC_SHA256", "FACELINK_CV_PYTHON", "FACELINK_CV_ENGINE_DIR", "FACELINK_CV_MODEL_DIR"}
	values := make([]string, len(names))
	var missing []string
	for i, name := range names {
		values[i] = os.Getenv(name)
		if values[i] == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == len(names) {
		t.Skipf("opt-in real-model synthetic integration not configured; missing %s; only a wholly procedural pinned JPEG is permitted", strings.Join(missing, ", "))
	}
	if len(missing) != 0 {
		t.Fatalf("partial opt-in real-model configuration; missing %s", strings.Join(missing, ", "))
	}
	pin, err := hex.DecodeString(values[1])
	if err != nil || len(pin) != sha256.Size {
		t.Fatal("FACELINK_CV_SYNTHETIC_SHA256 must be a SHA-256 hex digest")
	}
	for _, i := range []int{0, 2, 3, 4} {
		if !filepath.IsAbs(values[i]) {
			t.Fatalf("%s must be an explicit absolute path", names[i])
		}
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve known repository engine paths")
	}
	engine := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", "services", "face-engine"))
	for _, pair := range [][2]string{{values[3], engine}, {values[4], filepath.Join(engine, "models")}} {
		configured, e1 := os.Stat(pair[0])
		known, e2 := os.Stat(pair[1])
		if e1 != nil || e2 != nil || !configured.IsDir() || !os.SameFile(configured, known) {
			t.Fatal("real-model integration must use this repository's services/face-engine and models directories")
		}
	}
	python, err := os.Stat(values[2])
	if err != nil || !python.Mode().IsRegular() {
		t.Fatal("FACELINK_CV_PYTHON must identify an existing interpreter file")
	}
	file, err := os.Open(values[0])
	if err != nil {
		t.Fatal("cannot open explicitly configured procedural JPEG")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > enrollmentsecure.MaxImageBytes {
		t.Fatal("procedural JPEG must be a regular file within enrollment size limits")
	}
	jpg, err := io.ReadAll(io.LimitReader(file, enrollmentsecure.MaxImageBytes+1))
	if err != nil || len(jpg) > enrollmentsecure.MaxImageBytes {
		clear(jpg)
		t.Fatal("could not read bounded procedural JPEG")
	}
	hash := sha256.Sum256(jpg)
	if !bytes.Equal(hash[:], pin) {
		clear(jpg)
		t.Fatal("procedural JPEG SHA-256 does not match explicit pin")
	}
	processor, err := enrollmentcv.New(values[2], values[3], values[4])
	if err != nil {
		clear(jpg)
		t.Fatal("explicit real-model processor configuration is unavailable")
	}
	manifest := processor.Manifest()
	for _, model := range [][2]string{{"face_recognition_sface_2021dec.onnx", manifest.ModelChecksum}, {"face_detection_yunet_2023mar.onnx", manifest.DetectorChecksum}} {
		f, openErr := os.Open(filepath.Join(values[4], model[0]))
		if openErr != nil {
			clear(jpg)
			t.Fatal("required pinned real model is unavailable")
		}
		digest := sha256.New()
		_, copyErr := io.Copy(digest, f)
		closeErr := f.Close()
		if copyErr != nil || closeErr != nil || hex.EncodeToString(digest.Sum(nil)) != model[1] {
			clear(jpg)
			t.Fatal("real model does not match processor fingerprint")
		}
	}
	return jpg, processor
}

func realModelData(t *testing.T, response responsUji, want int) map[string]any {
	t.Helper()
	realModelPublicJSON(t, response.Body)
	if response.Status != want || response.Body["error"] != nil {
		t.Fatalf("integration API status=%d want=%d (response body intentionally omitted)", response.Status, want)
	}
	data, ok := response.Body["data"].(map[string]any)
	if !ok {
		t.Fatal("integration API omitted object data")
	}
	return data
}

func realModelPublicJSON(t *testing.T, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch strings.ToLower(key) {
			case "embedding", "embedding_vector", "vector", "ciphertext", "nonce", "aad", "jpeg_base64":
				t.Fatal("integration JSON exposed protected biometric material")
			}
			realModelPublicJSON(t, child)
		}
	case []any:
		for _, child := range value {
			realModelPublicJSON(t, child)
		}
	}
}

func realModelID(t *testing.T, data map[string]any) string {
	t.Helper()
	id, ok := data["resource_id"].(string)
	if !ok || id == "" {
		t.Fatal("integration mutation omitted resource ID")
	}
	return id
}

func realModelEnvelope(t *testing.T, envelope enrollmentsecure.Envelope, version, plaintextLength int) {
	t.Helper()
	if envelope.Version != version || envelope.Algorithm != "AES-256-GCM" || envelope.KeyVersion != "1" || len(envelope.Nonce) != 12 || len(envelope.Ciphertext) != plaintextLength+16 {
		t.Fatal("AES-GCM envelope version/algorithm/key-version/nonce/tag invariant failed")
	}
}

func realModelMustReject(t *testing.T, keys enrollmentsecure.KeyProvider, binding enrollmentsecure.Binding, envelope enrollmentsecure.Envelope) {
	t.Helper()
	plain, err := enrollmentsecure.Open(context.Background(), keys, binding, envelope)
	defer clear(plain)
	if !errors.Is(err, enrollmentsecure.ErrIntegrity) || len(plain) != 0 {
		t.Fatal("wrong binding/key or tampered envelope was not rejected with integrity failure")
	}
}

func realModelNoFiles(t *testing.T, files *enrollmentsecure.FileStore) {
	t.Helper()
	ids, err := files.IDs()
	if err != nil || len(ids) != 0 {
		t.Fatal("reconciliation left encrypted photo files")
	}
}

func realModelAudit(t *testing.T, h *harnessAuth, adminID, studentID, submission string) {
	t.Helper()
	rows, err := h.db.QueryContext(context.Background(), "SELECT action,actor_type,COALESCE(actor_id,''),COALESCE(entity_id,''),result,metadata_json FROM audit_events ORDER BY rowid")
	if err != nil {
		t.Fatal("cannot inspect integration audit invariants")
	}
	defer rows.Close()
	counts := map[string]int{}
	positions := map[string]int{}
	for index := 0; rows.Next(); index++ {
		var action, actorType, actorID, entity, result string
		var metadata sql.NullString
		if err = rows.Scan(&action, &actorType, &actorID, &entity, &result, &metadata); err != nil {
			t.Fatal("cannot read integration audit record")
		}
		switch action {
		case "consent.granted", "enrollment.submitted", "photo.preview_authorized", "enrollment.approved", "processing.queued", "processing.started", "photo.deleted", "enrollment.activated", "consent.withdrawn", "enrollment.revoked", "template.deleted", "deletion.requested", "deletion.completed":
			counts[action]++
			positions[action] = index
			if result != "success" {
				t.Fatal("enrollment lifecycle audit has a non-success result")
			}
			if action == "enrollment.approved" || action == "photo.preview_authorized" {
				if actorType != "user" || actorID != adminID || entity != submission {
					t.Fatal("review audit actor or submission mismatch")
				}
			}
			if action == "consent.granted" || action == "consent.withdrawn" || action == "enrollment.submitted" {
				if actorType != "user" || actorID != studentID {
					t.Fatal("student lifecycle audit actor mismatch")
				}
			}
			if action == "processing.started" || action == "enrollment.activated" {
				if actorType != "system" || actorID != "" || entity != submission {
					t.Fatal("real processing audit actor or submission mismatch")
				}
			}
			if metadata.Valid {
				var values map[string]string
				if json.Unmarshal([]byte(metadata.String), &values) != nil {
					t.Fatal("invalid lifecycle audit metadata")
				}
				for key := range values {
					if key != "reason_code" {
						t.Fatal("lifecycle audit contains non-allowlisted metadata")
					}
				}
			}
		case "processing.failed":
			t.Fatal("positive real-model transaction emitted processing.failed")
		}
	}
	if rows.Err() != nil {
		t.Fatal("integration audit iteration failed")
	}
	for _, action := range []string{"consent.granted", "enrollment.submitted", "photo.preview_authorized", "enrollment.approved", "processing.queued", "processing.started", "photo.deleted", "enrollment.activated", "consent.withdrawn", "enrollment.revoked", "template.deleted"} {
		if counts[action] != 1 {
			t.Fatalf("lifecycle audit action=%s count=%d want=1", action, counts[action])
		}
	}
	if counts["deletion.requested"] != 2 || counts["deletion.completed"] != 2 {
		t.Fatal("activation and withdrawal must each request and complete payload cleanup")
	}
	ordered := []string{"consent.granted", "enrollment.submitted", "photo.preview_authorized", "enrollment.approved", "processing.queued", "processing.started", "photo.deleted", "enrollment.activated", "consent.withdrawn", "enrollment.revoked", "template.deleted"}
	for i := 1; i < len(ordered); i++ {
		if positions[ordered[i-1]] >= positions[ordered[i]] {
			t.Fatal("lifecycle audit ordering invariant failed")
		}
	}
}
