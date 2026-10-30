// Package enrollmentcv runs the synthetic-only local YuNet/SFace worker.
package enrollmentcv

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unicode/utf8"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

const (
	maxJPEG          = 1 << 20
	maxRequest       = 1400000
	maxResponse      = 16384
	frameHeader      = 4 + sha256.Size
	inferenceTimeout = 10 * time.Second
)

var errProtocol = errors.New("invalid local inference response")

// Processor has one inference slot; each call uses a new child, key and request ID.
// Configuration paths are trusted operator input, never enrollment request input.
type Processor struct {
	pythonExecutable string
	engineDir        string
	modelDir         string
	inflight         chan struct{}
}

var _ enrollment.Processor = (*Processor)(nil)

func New(pythonExecutable, engineDir, modelDir string) (*Processor, error) {
	if pythonExecutable == "" || engineDir == "" || modelDir == "" {
		return nil, unavailable()
	}
	executable, err := exec.LookPath(pythonExecutable)
	if err != nil {
		return nil, unavailable()
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, unavailable()
	}
	engine, err := absoluteDirectory(engineDir)
	if err != nil {
		return nil, unavailable()
	}
	models, err := absoluteDirectory(modelDir)
	if err != nil {
		return nil, unavailable()
	}
	return &Processor{pythonExecutable: executable, engineDir: engine, modelDir: models, inflight: make(chan struct{}, 1)}, nil
}

func absoluteDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errProtocol
	}
	return absolute, nil
}

func (*Processor) Manifest() enrollment.Manifest {
	return enrollment.Manifest{
		SyntheticOnly: true,
		ModelID:       "opencv-sface", ModelVersion: "2021dec",
		ModelChecksum: "0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79",
		DetectorID:    "opencv-yunet", DetectorVersion: "2023mar",
		DetectorChecksum: "8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4",
		PipelineRevision: "enr04-synthetic-native-v2",
	}
}

func unavailable() error      { return &enrollment.ProcessingError{Code: "MODEL_UNAVAILABLE"} }
func extractionFailed() error { return &enrollment.ProcessingError{Code: "EXTRACTION_FAILED"} }

type request struct {
	Version        int    `json:"version"`
	RequestID      string `json:"request_id"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	JPEGBase64     string `json:"jpeg_base64"`
}

type wireManifest struct {
	SyntheticOnly    bool   `json:"synthetic_only"`
	ModelID          string `json:"model_id"`
	ModelVersion     string `json:"model_version"`
	ModelChecksum    string `json:"model_checksum"`
	DetectorID       string `json:"detector_id"`
	DetectorVersion  string `json:"detector_version"`
	DetectorChecksum string `json:"detector_checksum"`
	PipelineRevision string `json:"pipeline_revision"`
}

func (m wireManifest) enrollmentManifest() enrollment.Manifest {
	return enrollment.Manifest{SyntheticOnly: m.SyntheticOnly, ModelID: m.ModelID, ModelVersion: m.ModelVersion, ModelChecksum: m.ModelChecksum, DetectorID: m.DetectorID, DetectorVersion: m.DetectorVersion, DetectorChecksum: m.DetectorChecksum, PipelineRevision: m.PipelineRevision}
}

type response struct {
	Version        int          `json:"version"`
	RequestID      string       `json:"request_id"`
	ArtifactSHA256 string       `json:"artifact_sha256"`
	Status         string       `json:"status"`
	FaceCount      int          `json:"face_count"`
	Embedding      []float32    `json:"embedding"`
	Manifest       wireManifest `json:"manifest"`
}

func (p *Processor) Process(ctx context.Context, jpeg []byte) (enrollment.ProcessingResult, error) {
	var empty enrollment.ProcessingResult
	if p == nil || p.inflight == nil {
		return empty, unavailable()
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	select {
	case p.inflight <- struct{}{}:
		defer func() { <-p.inflight }()
	default:
		return empty, unavailable()
	}
	if len(jpeg) == 0 || len(jpeg) > maxJPEG {
		return empty, extractionFailed()
	}
	childCtx, cancel := context.WithTimeout(ctx, inferenceTimeout)
	defer cancel()
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return empty, unavailable()
	}
	defer clear(secret[:])
	digest := sha256.Sum256(jpeg)
	req := request{Version: 1, RequestID: ids.New(), ArtifactSHA256: hex.EncodeToString(digest[:]), JPEGBase64: base64.StdEncoding.EncodeToString(jpeg)}
	payload, err := json.Marshal(req)
	if err != nil || len(payload) > maxRequest {
		return empty, extractionFailed()
	}
	defer clear(payload)
	frame := encodeFrame(payload, secret[:])
	defer clear(frame)
	cmd := exec.CommandContext(childCtx, p.pythonExecutable, "-m", "face_engine.ipc", "--model-dir", p.modelDir)
	cmd.Dir = p.engineDir
	cmd.Env = childEnvironment(secret[:])
	// os/exec closes the pipe at EOF, allowing the child to reject trailing input.
	cmd.Stdin = bytes.NewReader(frame)
	output := boundedOutput{limit: frameHeader + maxResponse, cancel: cancel}
	defer func() { clear(output.data) }()
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	// Bound pipe cleanup even if a broken worker leaves inherited handles open.
	cmd.WaitDelay = time.Second
	err = cmd.Run() // Run always waits/reaps, including CommandContext cancellation.
	clear(cmd.Env)
	if childCtx.Err() != nil {
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		if !output.exceeded {
			return empty, context.DeadlineExceeded
		}
	}
	if err != nil || output.exceeded {
		return empty, unavailable()
	}
	body, err := decodeFrame(output.data, secret[:])
	if err != nil {
		return empty, unavailable()
	}
	return p.decodeResponse(body, req)
}

// Keep only Windows runtime essentials; never forward credentials, PYTHONPATH,
// PYTHONHOME, user-site configuration, or arbitrary loader/network settings.
func childEnvironment(secret []byte) []string {
	env := []string{"FACELINK_CV_SECRET=" + hex.EncodeToString(secret), "PYTHONNOUSERSITE=1", "PYTHONDONTWRITEBYTECODE=1", "OMP_NUM_THREADS=1", "OPENBLAS_NUM_THREADS=1", "MKL_NUM_THREADS=1"}
	for _, key := range []string{"SystemRoot", "WINDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

type boundedOutput struct {
	data     []byte
	limit    int
	exceeded bool
	cancel   context.CancelFunc
}

func (w *boundedOutput) Write(b []byte) (int, error) {
	if len(b) > w.limit-len(w.data) {
		w.exceeded = true
		w.cancel()
		return 0, errProtocol
	}
	w.data = append(w.data, b...)
	return len(b), nil
}

func encodeFrame(payload, secret []byte) []byte {
	frame := make([]byte, frameHeader+len(payload))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	copy(frame[4:frameHeader], mac.Sum(nil))
	copy(frame[frameHeader:], payload)
	return frame
}

func decodeFrame(frame, secret []byte) ([]byte, error) {
	if len(frame) < frameHeader {
		return nil, errProtocol
	}
	n := binary.BigEndian.Uint32(frame[:4])
	if n == 0 || n > maxResponse || int(n) != len(frame)-frameHeader {
		return nil, errProtocol
	}
	payload := frame[frameHeader:]
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(frame[4:frameHeader], mac.Sum(nil)) {
		return nil, errProtocol
	}
	return payload, nil
}

// encoding/json alone accepts duplicate keys and case-insensitive field names.
// Require the exact non-null object first, including the nested manifest.
func exactObject(payload []byte, fields ...string) (map[string]json.RawMessage, error) {
	if !utf8.Valid(payload) {
		return nil, errProtocol
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errProtocol
	}
	values := make(map[string]json.RawMessage, len(fields))
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return nil, errProtocol
		}
		key, ok := token.(string)
		if !ok {
			return nil, errProtocol
		}
		if _, exists := values[key]; exists {
			return nil, errProtocol
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(value, []byte("null")) {
			return nil, errProtocol
		}
		values[key] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, errProtocol
	}
	if decoder.Decode(new(json.RawMessage)) != io.EOF || len(values) != len(fields) {
		return nil, errProtocol
	}
	for _, key := range fields {
		if _, exists := values[key]; !exists {
			return nil, errProtocol
		}
	}
	return values, nil
}

func numberArray(payload []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return false
	}
	count := 0
	for decoder.More() {
		token, err = decoder.Token()
		if _, ok := token.(float64); err != nil || !ok {
			return false
		}
		count++
		if count > 128 {
			return false
		}
	}
	token, err = decoder.Token()
	return err == nil && token == json.Delim(']') && decoder.Decode(new(json.RawMessage)) == io.EOF
}

func (p *Processor) decodeResponse(payload []byte, req request) (enrollment.ProcessingResult, error) {
	var empty enrollment.ProcessingResult
	fields, err := exactObject(payload, "version", "request_id", "artifact_sha256", "status", "face_count", "embedding", "manifest")
	if err != nil {
		return empty, unavailable()
	}
	if _, err = exactObject(fields["manifest"], "synthetic_only", "model_id", "model_version", "model_checksum", "detector_id", "detector_version", "detector_checksum", "pipeline_revision"); err != nil {
		return empty, unavailable()
	}
	if !numberArray(fields["embedding"]) {
		return empty, unavailable()
	}
	var result response
	if err = json.Unmarshal(payload, &result); err != nil {
		return empty, unavailable()
	}
	if result.Version != 1 || result.RequestID != req.RequestID || result.ArtifactSHA256 != req.ArtifactSHA256 || result.Manifest.enrollmentManifest() != p.Manifest() || result.FaceCount < 0 || result.FaceCount > 1024 {
		clear(result.Embedding)
		return empty, unavailable()
	}
	if result.Status != "OK" {
		if len(result.Embedding) != 0 || (result.Status == "NO_FACE" && result.FaceCount != 0) || (result.Status == "MULTIPLE_FACES" && result.FaceCount < 2) {
			clear(result.Embedding)
			return empty, unavailable()
		}
		switch result.Status {
		case "NO_FACE", "MULTIPLE_FACES", "MODEL_UNAVAILABLE", "LOW_QUALITY", "FACE_TOO_SMALL", "INVALID_IMAGE", "INFERENCE_FAILED":
			return empty, &enrollment.ProcessingError{Code: result.Status}
		default:
			return empty, unavailable()
		}
	}
	if result.FaceCount != 1 || len(result.Embedding) != 128 {
		clear(result.Embedding)
		return empty, unavailable()
	}
	var norm float64
	for _, value := range result.Embedding {
		v := float64(value)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			clear(result.Embedding)
			return empty, unavailable()
		}
		norm += v * v
	}
	if math.Abs(math.Sqrt(norm)-1) > 1e-5 {
		clear(result.Embedding)
		return empty, unavailable()
	}
	return enrollment.ProcessingResult{Embedding: result.Embedding, FaceCount: 1, Manifest: p.Manifest()}, nil
}
