//go:build !windows

package otpkey

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMenolakPermissionTerbukaDanRepository(t *testing.T) {
	key := bytes.Repeat([]byte{17}, 32)
	dir := t.TempDir()
	p := filepath.Join(dir, "otp.key")
	if err := os.WriteFile(p, []byte(base64.StdEncoding.EncodeToString(key)), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, key) {
		t.Fatal("key berbeda")
	}
	if err := os.Chmod(p, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("key world-readable diterima")
	}
	if err := os.Chmod(p, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("key dalam repository diterima")
	}
}
