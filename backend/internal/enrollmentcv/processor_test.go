package enrollmentcv

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
)

// The test executable stands in for Python only in subprocess protocol tests.
// Production always executes face_engine.ipc and never has a fallback processor.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "-m" {
		os.Exit(runProtocolHelper())
	}
	os.Exit(m.Run())
}

func runProtocolHelper() int {
	if len(os.Args) != 5 || os.Args[2] != "face_engine.ipc" || os.Args[3] != "--model-dir" || !filepath.IsAbs(os.Args[4]) {
		return 21
	}
	for _, name := range []string{"FACELINK_UNRELATED_SECRET", "PYTHONPATH", "PYTHONHOME", "AWS_SECRET_ACCESS_KEY", "HOME"} {
		if os.Getenv(name) != "" {
			return 22
		}
	}
	secret, err := hex.DecodeString(os.Getenv("FACELINK_CV_SECRET"))
	if err != nil || len(secret) != 32 || os.Getenv("PYTHONNOUSERSITE") != "1" || os.Getenv("PYTHONDONTWRITEBYTECODE") != "1" {
		return 23
	}
	input, err := io.ReadAll(io.LimitReader(os.Stdin, maxRequest+frameHeader+1))
	if err != nil || len(input) < frameHeader || len(input) > maxRequest+frameHeader || int(binary.BigEndian.Uint32(input[:4])) != len(input)-frameHeader {
		return 24
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(input[frameHeader:])
	if !hmac.Equal(input[4:frameHeader], mac.Sum(nil)) {
		return 25
	}
	var req request
	if json.Unmarshal(input[frameHeader:], &req) != nil || req.Version != 1 || len(req.RequestID) != 36 || req.RequestID[14] != '4' || !strings.ContainsRune("89ab", rune(req.RequestID[19])) {
		return 26
	}
	jpegBytes, err := base64.StdEncoding.Strict().DecodeString(req.JPEGBase64)
	digest := sha256.Sum256(jpegBytes)
	if err != nil || req.ArtifactSHA256 != hex.EncodeToString(digest[:]) {
		return 27
	}
	mode := filepath.Base(os.Args[4])
	if mode == "crash" {
		_, _ = os.Stderr.WriteString("SECRET_PRIVATE_MODEL_PATH traceback image payload must never escape")
		return 17
	}
	if mode == "hang" || mode == "controlled" {
		if err := os.WriteFile(filepath.Join(os.Args[4], "entered"), []byte("ready"), 0600); err != nil {
			return 28
		}
		if mode == "hang" {
			time.Sleep(30 * time.Second)
		} else {
			for {
				if _, err := os.Stat(filepath.Join(os.Args[4], "release")); err == nil {
					break
				}
				time.Sleep(time.Millisecond)
			}
		}
	}
	manifest := (&Processor{}).Manifest()
	result := response{Version: 1, RequestID: req.RequestID, ArtifactSHA256: req.ArtifactSHA256, Status: "OK", FaceCount: 1, Embedding: make([]float32, 128), Manifest: wireManifest{SyntheticOnly: manifest.SyntheticOnly, ModelID: manifest.ModelID, ModelVersion: manifest.ModelVersion, ModelChecksum: manifest.ModelChecksum, DetectorID: manifest.DetectorID, DetectorVersion: manifest.DetectorVersion, DetectorChecksum: manifest.DetectorChecksum, PipelineRevision: manifest.PipelineRevision}}
	result.Embedding[0] = 1
	switch mode {
	case "version":
		result.Version = 2
	case "nonce":
		result.RequestID = "00000000-0000-4000-8000-000000000000"
	case "digest":
		result.ArtifactSHA256 = strings.Repeat("0", 64)
	case "manifest":
		result.Manifest.ModelChecksum = strings.Repeat("0", 64)
	case "synthetic":
		result.Manifest.SyntheticOnly = false
	case "dimension":
		result.Embedding = result.Embedding[:127]
	case "norm":
		result.Embedding[0] = 2
	case "zero":
		result.Embedding[0] = 0
	case "negative-count":
		result.FaceCount = -1
	case "large-count":
		result.FaceCount = 1025
	case "ok-no-face":
		result.FaceCount = 0
	case "unknown-status":
		result.Status = "PRIVATE_TRACEBACK"
		result.Embedding = []float32{}
	case "failure-vector":
		result.Status = "NO_FACE"
		result.FaceCount = 0
	case "inconsistent-no-face":
		result.Status = "NO_FACE"
		result.Embedding = []float32{}
	case "inconsistent-multiple":
		result.Status = "MULTIPLE_FACES"
		result.Embedding = []float32{}
	default:
		if strings.HasPrefix(mode, "status-") {
			result.Status = strings.TrimPrefix(mode, "status-")
			result.FaceCount = 0
			if result.Status == "MULTIPLE_FACES" {
				result.FaceCount = 2
			}
			result.Embedding = []float32{}
		}
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return 29
	}
	switch mode {
	case "unknown-field":
		payload = append([]byte(`{"unknown":true,`), payload[1:]...)
	case "duplicate-field":
		payload = append([]byte(`{"version":1,`), payload[1:]...)
	case "case-field":
		payload = bytes.Replace(payload, []byte(`"version"`), []byte(`"Version"`), 1)
	case "missing-field":
		payload = bytes.Replace(payload, []byte(`"version":1,`), nil, 1)
	case "null-field":
		payload = bytes.Replace(payload, []byte(`"version":1`), []byte(`"version":null`), 1)
	case "unknown-manifest":
		payload = bytes.Replace(payload, []byte(`"manifest":{`), []byte(`"manifest":{"unknown":true,`), 1)
	case "duplicate-manifest":
		payload = bytes.Replace(payload, []byte(`"manifest":{`), []byte(`"manifest":{"synthetic_only":true,`), 1)
	case "null-element":
		payload = bytes.Replace(payload, []byte(`"embedding":[1,0`), []byte(`"embedding":[1,null`), 1)
	case "string-element":
		payload = bytes.Replace(payload, []byte(`"embedding":[1,0`), []byte(`"embedding":[1,"0"`), 1)
	case "nan":
		payload = bytes.Replace(payload, []byte(`"embedding":[1`), []byte(`"embedding":[NaN`), 1)
	case "infinity":
		payload = bytes.Replace(payload, []byte(`"embedding":[1`), []byte(`"embedding":[1e999`), 1)
	case "float32-overflow":
		payload = bytes.Replace(payload, []byte(`"embedding":[1`), []byte(`"embedding":[1e39`), 1)
	case "trailing-json":
		payload = append(payload, []byte(` {}`)...)
	case "malformed":
		payload = []byte(`{`)
	case "invalid-utf8":
		payload = append([]byte{0xff}, payload...)
	}
	frame := encodeFrame(payload, secret)
	switch mode {
	case "auth":
		frame[4] ^= 1
	case "truncated":
		frame = frame[:len(frame)-1]
	case "short-header":
		frame = frame[:7]
	case "trailing-frame":
		frame = append(frame, 0)
	case "empty-frame":
		frame = encodeFrame(nil, secret)
	case "length":
		binary.BigEndian.PutUint32(frame[:4], maxResponse+1)
	case "oversize":
		frame = bytes.Repeat([]byte("x"), maxResponse*4)
	}
	_, err = os.Stdout.Write(frame)
	if err != nil {
		return 30
	}
	return 0
}

func helperProcessor(t *testing.T, mode string) *Processor {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	models := filepath.Join(root, mode)
	if err = os.Mkdir(models, 0700); err != nil {
		t.Fatal(err)
	}
	processor, err := New(executable, root, models)
	if err != nil {
		t.Fatal(err)
	}
	return processor
}

func syntheticJPEG(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := jpeg.Encode(&output, image.NewGray(image.Rect(0, 0, 32, 32)), nil); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func expectProcessingCode(t *testing.T, err error, want string) {
	t.Helper()
	var processing *enrollment.ProcessingError
	if !errors.As(err, &processing) || processing.Code != want || err.Error() != want {
		t.Fatalf("expected sanitized %s, got %v", want, err)
	}
}

func TestAuthenticatedProcessAndMinimalEnvironment(t *testing.T) {
	t.Setenv("FACELINK_UNRELATED_SECRET", "must-not-reach-child")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "must-not-reach-child")
	t.Setenv("FACELINK_CV_SECRET", "must-not-be-reused")
	t.Setenv("PYTHONPATH", "must-not-reach-child")
	t.Setenv("PYTHONHOME", "must-not-reach-child")
	p := helperProcessor(t, "valid")
	result, err := p.Process(context.Background(), syntheticJPEG(t))
	if err != nil {
		t.Fatal(err)
	}
	if result.Manifest != p.Manifest() || result.FaceCount != 1 || len(result.Embedding) != 128 || result.Embedding[0] != 1 {
		t.Fatal("valid authenticated result was not preserved")
	}
	if !filepath.IsAbs(p.engineDir) || !filepath.IsAbs(p.modelDir) || !filepath.IsAbs(p.pythonExecutable) {
		t.Fatal("runtime configuration was not anchored")
	}
}

func TestProtocolAndProcessFailuresClose(t *testing.T) {
	for _, mode := range []string{"auth", "truncated", "short-header", "trailing-frame", "empty-frame", "length", "oversize", "crash", "version", "nonce", "digest", "manifest", "synthetic", "dimension", "norm", "zero", "negative-count", "large-count", "ok-no-face", "unknown-status", "failure-vector", "inconsistent-no-face", "inconsistent-multiple", "unknown-field", "duplicate-field", "case-field", "missing-field", "null-field", "unknown-manifest", "duplicate-manifest", "null-element", "string-element", "nan", "infinity", "float32-overflow", "trailing-json", "malformed", "invalid-utf8"} {
		t.Run(mode, func(t *testing.T) {
			p := helperProcessor(t, mode)
			result, err := p.Process(context.Background(), syntheticJPEG(t))
			expectProcessingCode(t, err, "MODEL_UNAVAILABLE")
			if len(result.Embedding) != 0 || result.FaceCount != 0 || result.Manifest != (enrollment.Manifest{}) {
				t.Fatal("untrusted output escaped failure")
			}
		})
	}
}

func TestAllowlistedStructuredFailures(t *testing.T) {
	for _, status := range []string{"NO_FACE", "MULTIPLE_FACES", "LOW_QUALITY", "FACE_TOO_SMALL", "INVALID_IMAGE", "MODEL_UNAVAILABLE", "INFERENCE_FAILED"} {
		t.Run(status, func(t *testing.T) {
			p := helperProcessor(t, "status-"+status)
			result, err := p.Process(context.Background(), syntheticJPEG(t))
			expectProcessingCode(t, err, status)
			if len(result.Embedding) != 0 {
				t.Fatal("failure returned a vector")
			}
		})
	}
}

func awaitHelper(t *testing.T, p *Processor) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(filepath.Join(p.modelDir, "entered")); err == nil {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("helper never entered inference")
		case <-ticker.C:
		}
	}
}

func awaitResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("canceled child was not reaped promptly")
		return nil
	}
}

func TestCancellationKillsReapsAndReleasesSlot(t *testing.T) {
	p := helperProcessor(t, "hang")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	photo := syntheticJPEG(t)
	done := make(chan error, 1)
	go func() { _, err := p.Process(ctx, photo); done <- err }()
	awaitHelper(t, p)
	cancel()
	if err := awaitResult(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was not retained: %v", err)
	}
	if len(p.inflight) != 0 {
		t.Fatal("reaped process retained slot")
	}
	p.modelDir = filepath.Join(filepath.Dir(p.modelDir), "valid")
	if _, err := p.Process(context.Background(), photo); err != nil {
		t.Fatalf("slot could not process after cancellation: %v", err)
	}
}

func TestDeadlineKillsReapsChild(t *testing.T) {
	p := helperProcessor(t, "hang")
	photo := syntheticJPEG(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := p.Process(ctx, photo); done <- err }()
	awaitHelper(t, p)
	if err := awaitResult(t, done); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline was not retained: %v", err)
	}
	if len(p.inflight) != 0 {
		t.Fatal("timed-out process retained slot")
	}
}

func TestSingleInflightProcessor(t *testing.T) {
	p := helperProcessor(t, "controlled")
	photo := syntheticJPEG(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := p.Process(ctx, photo); done <- err }()
	awaitHelper(t, p)
	_, err := p.Process(context.Background(), photo)
	expectProcessingCode(t, err, "MODEL_UNAVAILABLE")
	if err = os.WriteFile(filepath.Join(p.modelDir, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err = awaitResult(t, done); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Process(context.Background(), photo); err != nil {
		t.Fatalf("slot not released: %v", err)
	}
}

func TestInputAndConfigurationBounds(t *testing.T) {
	p := helperProcessor(t, "valid")
	for _, photo := range [][]byte{nil, make([]byte, maxJPEG+1)} {
		_, err := p.Process(context.Background(), photo)
		expectProcessingCode(t, err, "EXTRACTION_FAILED")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Process(ctx, syntheticJPEG(t)); !errors.Is(err, context.Canceled) {
		t.Fatal("pre-canceled work was not rejected")
	}
	if _, err := New("", p.engineDir, p.modelDir); err == nil {
		t.Fatal("missing executable accepted")
	}
	if _, err := New(p.pythonExecutable, filepath.Join(p.engineDir, "missing"), p.modelDir); err == nil {
		t.Fatal("missing engine accepted")
	}
	file := filepath.Join(p.engineDir, "not-a-directory")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(p.pythonExecutable, p.engineDir, file); err == nil {
		t.Fatal("non-directory models accepted")
	}
}
