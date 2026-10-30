package enrollmentcv

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/enrollment"
)

// This is a real process-launch failure, not real-model inference or a
// fabricated successful IPC response. A disappearing executable must never
// produce an embedding, even after configuration was accepted.
func TestPythonExecutableUnavailableBeforeAndAfterConstruction(t *testing.T) {
	p := helperProcessor(t, "valid")
	missing := filepath.Join(t.TempDir(), "missing-python-executable")
	_, err := New(missing, p.engineDir, p.modelDir)
	expectProcessingCode(t, err, "MODEL_UNAVAILABLE")
	p.pythonExecutable = missing
	result, err := p.Process(context.Background(), syntheticJPEG(t))
	expectProcessingCode(t, err, "MODEL_UNAVAILABLE")
	if len(result.Embedding) != 0 || result.FaceCount != 0 || result.Manifest != (enrollment.Manifest{}) {
		t.Fatal("unavailable executable returned trusted inference output")
	}
}
