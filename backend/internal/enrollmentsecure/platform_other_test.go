//go:build !windows

package enrollmentsecure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistentKeysNeverFallbackOutsideWindows(t *testing.T) {
	root := filepath.Join(t.TempDir(), "keys")
	if Provision(root) != ErrUnavailable {
		t.Fatal("non-Windows persistent key fallback enabled")
	}
	if _, err := LoadKeyProvider(root); err != ErrUnavailable {
		t.Fatal("non-Windows persistent keys accepted")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("unsupported platform touched key directory")
	}
}
func TestRejectBroadPermissionsAndHardlinks(t *testing.T) {
	f := testStore(t)
	if err := f.Stage(testFileID, testEnvelope(t)); err != nil {
		t.Fatal(err)
	}
	if err := f.Commit(testFileID); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.root, testFileID+".enc")
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Read(testFileID); err == nil {
		t.Fatal("broad permissions accepted")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, filepath.Join(t.TempDir(), "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Read(testFileID); err == nil {
		t.Fatal("hardlink accepted")
	}
	if err := os.Chmod(f.root, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := f.IDs(); err == nil {
		t.Fatal("broad root permissions accepted")
	}
}
