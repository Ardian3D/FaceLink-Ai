package enrollmentsecure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const testFileID = "11111111-2222-4333-8444-555555555555"

func testStore(t *testing.T) *FileStore {
	if !fileStoreSupported {
		t.Skip("atomic no-replace file storage is unsupported on this platform")
	}
	t.Helper()
	f, err := NewFileStore(filepath.Join(t.TempDir(), "ciphertext"))
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func testEnvelope(t *testing.T) Envelope {
	t.Helper()
	e, err := Seal(context.Background(), newTestKeys(), photoBinding(), []byte("synthetic-only-private-payload"))
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func TestCiphertextStoreStageCommitReadDelete(t *testing.T) {
	f := testStore(t)
	e := testEnvelope(t)
	if err := f.Stage(testFileID, e); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Read(testFileID); err == nil {
		t.Fatal("uncommitted stage exposed")
	}
	ids, err := f.IDs()
	if err != nil || len(ids) != 1 || ids[0] != testFileID {
		t.Fatal("stage absent from reconciliation")
	}
	staged, err := os.ReadFile(filepath.Join(f.root, testFileID+".stage"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(staged, []byte("synthetic-only-private-payload")) {
		t.Fatal("plaintext on disk")
	}
	if err := f.Stage(testFileID, e); err == nil {
		t.Fatal("stage overwritten")
	}
	if err := f.Commit(testFileID); err != nil {
		t.Fatal(err)
	}
	got, err := f.Read(testFileID)
	if err != nil || !bytes.Equal(got.Ciphertext, e.Ciphertext) {
		t.Fatal("ciphertext roundtrip failed")
	}
	if err := f.Commit(testFileID); err == nil {
		t.Fatal("missing stage accepted")
	}
	if err := f.Stage(testFileID, e); err == nil {
		t.Fatal("committed ciphertext overwritten")
	}
	for range 2 {
		if err := f.Delete(testFileID); err != nil {
			t.Fatal(err)
		}
	}
	ids, err = f.IDs()
	if err != nil || len(ids) != 0 {
		t.Fatal("delete left payloads")
	}
}
func TestFileStoreRefusesEscapeAndUnexpectedFiles(t *testing.T) {
	f := testStore(t)
	e := testEnvelope(t)
	for _, id := range []string{"../outside", "", testFileID + ".enc", "C:\\escape", "11111111-2222-4333-8444-55555555555A"} {
		if f.Stage(id, e) == nil || f.Commit(id) == nil || f.Delete(id) == nil {
			t.Fatal("unsafe reference accepted")
		}
		if _, err := f.Read(id); err == nil {
			t.Fatal("unsafe read accepted")
		}
	}
	if err := os.WriteFile(filepath.Join(f.root, "unexpected"), []byte("ciphertext"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.IDs(); !errors.Is(err, ErrIntegrity) {
		t.Fatal("unexpected file silently ignored")
	}
}
func TestFileStoreFailureAndCrashReconciliation(t *testing.T) {
	f := testStore(t)
	tmp := filepath.Join(f.root, testFileID+".tmp")
	interrupted, err := createPrivateFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate interruption immediately after CREATE_NEW, before any payload write
	// or separate ACL operation. Recovery must accept the empty private orphan.
	if err := interrupted.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewFileStore(f.root)
	if err != nil {
		t.Fatal(err)
	}
	f = restarted
	ids, err := f.IDs()
	if err != nil || len(ids) != 1 {
		t.Fatal("interrupted stage not reconcilable")
	}
	if f.Stage(testFileID, testEnvelope(t)) == nil {
		t.Fatal("interrupted stage replaced")
	}
	if f.Commit(testFileID) == nil {
		t.Fatal("partial tmp committed")
	}
	if err := f.Delete(testFileID); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(f.root, testFileID+".stage")
	if err := os.WriteFile(stage, []byte("{\"ciphertext\":"), 0600); err != nil {
		t.Fatal(err)
	}
	if protectPath(stage, false) != nil {
		t.Fatal("protection failed")
	}
	if f.Commit(testFileID) == nil {
		t.Fatal("malformed stage committed")
	}
	if err := f.Delete(testFileID); err != nil {
		t.Fatal(err)
	}
	if err := f.Stage(testFileID, testEnvelope(t)); err != nil {
		t.Fatal(err)
	}
	final := filepath.Join(f.root, testFileID+".enc")
	if err := os.WriteFile(final, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if protectPath(final, false) != nil {
		t.Fatal("protection failed")
	}
	if f.Commit(testFileID) == nil {
		t.Fatal("commit overwrote existing ciphertext")
	}
	data, err := os.ReadFile(final)
	if err != nil || string(data) != "existing" {
		t.Fatal("existing destination changed")
	}
	if _, err := f.Read(testFileID); !errors.Is(err, ErrIntegrity) {
		t.Fatal("malformed ciphertext read accepted")
	}
}
func TestFileStoreRejectsRepositoryAndRootReplacement(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileStore(filepath.Join(repo, "ciphertext")); err == nil {
		t.Fatal("repository storage accepted")
	}
	f := testStore(t)
	// Do not call ready/SameFile on the original store before replacement:
	// Windows Lstat identities would otherwise be materialized accidentally.
	if err := os.Rename(f.root, f.root+"-old"); err != nil {
		t.Fatal(err)
	}
	replacement, err := NewFileStore(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if f.Stage(testFileID, testEnvelope(t)) == nil {
		t.Fatal("replaced root accepted")
	}
	if ids, err := replacement.IDs(); err != nil || len(ids) != 0 {
		t.Fatal("old store wrote into replacement root")
	}
	if err := replacement.Stage(testFileID, testEnvelope(t)); err != nil {
		t.Fatal(err)
	}
	if err := replacement.Commit(testFileID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Read(testFileID); !errors.Is(err, ErrUnavailable) {
		t.Fatal("old store read replacement ciphertext")
	}
	if _, err := f.IDs(); !errors.Is(err, ErrUnavailable) {
		t.Fatal("old store enumerated replacement root")
	}
	if err := f.Commit(testFileID); !errors.Is(err, ErrUnavailable) {
		t.Fatal("old store accepted replacement root for commit")
	}
	if err := f.Delete(testFileID); !errors.Is(err, ErrUnavailable) {
		t.Fatal("old store deleted replacement ciphertext")
	}
	if _, err := replacement.Read(testFileID); err != nil {
		t.Fatal("old store altered replacement ciphertext")
	}
}
func TestFileStoreRejectsSymlink(t *testing.T) {
	f := testStore(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("do not delete"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(f.root, testFileID+".enc")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("host does not permit unprivileged symlink creation")
	}
	if _, err := f.Read(testFileID); err == nil {
		t.Fatal("symlink read accepted")
	}
	if _, err := f.IDs(); err == nil {
		t.Fatal("symlink listed")
	}
	if f.Delete(testFileID) == nil {
		t.Fatal("symlink deletion accepted")
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "do not delete" {
		t.Fatal("outside target altered")
	}
	if _, err := NewFileStore(link); err == nil {
		t.Fatal("symlink root accepted")
	}
}
func TestFileStoreRejectsOversizeCiphertext(t *testing.T) {
	f := testStore(t)
	path := filepath.Join(f.root, testFileID+".enc")
	if err := os.WriteFile(path, make([]byte, maxEnvelopeBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if protectPath(path, false) != nil {
		t.Fatal("protection failed")
	}
	if _, err := f.Read(testFileID); !errors.Is(err, ErrIntegrity) {
		t.Fatal("oversize storage accepted")
	}
}
