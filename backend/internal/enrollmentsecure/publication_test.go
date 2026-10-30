package enrollmentsecure

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writePrivateFixture(t *testing.T, path string, b []byte) {
	t.Helper()
	file, err := createPrivateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(b); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRestartImmediatelyAfterBothPublicationTransitions(t *testing.T) {
	for _, transition := range []struct{ from, to string }{{".tmp", ".stage"}, {".stage", ".enc"}} {
		t.Run(transition.from+" to "+transition.to, func(t *testing.T) {
			f := testStore(t)
			envelope := testEnvelope(t)
			encoded, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			from, to := filepath.Join(f.root, testFileID+transition.from), filepath.Join(f.root, testFileID+transition.to)
			writePrivateFixture(t, from, encoded)
			// Simulate restart immediately after the publication syscall, before the
			// caller's directory sync. There must be one name/inode, never two aliases.
			if err := moveNoReplace(from, to); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(from); !os.IsNotExist(err) {
				t.Fatal("publication retained a source alias")
			}
			restarted, err := NewFileStore(f.root)
			if err != nil {
				t.Fatal(err)
			}
			ids, err := restarted.IDs()
			if err != nil || len(ids) != 1 || ids[0] != testFileID {
				t.Fatal("published orphan not recoverable after restart")
			}
			if err := restarted.checkFile(to, false); err != nil {
				t.Fatal("publication created an unsafe hardlink state")
			}
			if transition.to == ".stage" {
				if err := restarted.Commit(testFileID); err != nil {
					t.Fatal(err)
				}
			}
			got, err := restarted.Read(testFileID)
			if err != nil || !bytes.Equal(got.Ciphertext, envelope.Ciphertext) {
				t.Fatal("published ciphertext lost on restart")
			}
			if err := restarted.Delete(testFileID); err != nil {
				t.Fatal(err)
			}
			// A recovered orphan must not poison the next reconciliation/operation.
			if err := restarted.Stage(testFileID, envelope); err != nil {
				t.Fatal(err)
			}
			if err := restarted.Commit(testFileID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBothPublicationTransitionsPreserveExistingDestination(t *testing.T) {
	for _, transition := range []struct{ from, to string }{{".tmp", ".stage"}, {".stage", ".enc"}} {
		t.Run(transition.from+" to "+transition.to, func(t *testing.T) {
			f := testStore(t)
			from, to := filepath.Join(f.root, testFileID+transition.from), filepath.Join(f.root, testFileID+transition.to)
			source, destination := []byte("synthetic source ciphertext"), []byte("synthetic destination ciphertext")
			writePrivateFixture(t, from, source)
			writePrivateFixture(t, to, destination)
			if err := moveNoReplace(from, to); err == nil {
				t.Fatal("publication replaced existing destination")
			}
			for path, want := range map[string][]byte{from: source, to: destination} {
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatal("collision changed source or destination")
				}
				if err := f.checkFile(path, false); err != nil {
					t.Fatal("collision created an unsafe hardlink state")
				}
			}
			restarted, err := NewFileStore(f.root)
			if err != nil {
				t.Fatal(err)
			}
			ids, err := restarted.IDs()
			if err != nil || len(ids) != 1 {
				t.Fatal("collision not recoverable after restart")
			}
			if err := restarted.Delete(testFileID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUnsupportedFileStoreFailsWithoutCreatingRoot(t *testing.T) {
	if fileStoreSupported {
		t.Skip("platform supports atomic publication")
	}
	root := filepath.Join(t.TempDir(), "unsupported-store")
	if _, err := NewFileStore(root); err != ErrUnavailable {
		t.Fatal("unsupported publication did not fail closed")
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("unsupported storage created a root")
	}
}
