//go:build windows

package otpkey

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"
)

func TestLoadKeyDPAPIDanTolakPlaintext(t *testing.T) {
	key := bytes.Repeat([]byte{17}, 32)
	in := dataBlob{Size: uint32(len(key)), Data: &key[0]}
	var out dataBlob
	protect := crypt32.NewProc("CryptProtectData")
	ok, _, _ := protect.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(key)
	if ok == 0 {
		t.Fatal("DPAPI tidak tersedia")
	}
	defer localFree.Call(uintptr(unsafe.Pointer(out.Data)))
	encrypted := append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...)
	path := filepath.Join(t.TempDir(), "otp.key")
	if err := os.WriteFile(path, encrypted, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, key) {
		t.Fatal("key yang dimuat berbeda")
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("key plaintext Windows diterima")
	}
}

func TestLoadKeyMenolakRepository(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "otp.key")
	if err := os.WriteFile(path, []byte("not-a-real-key"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("key di repository diterima")
	}
}
