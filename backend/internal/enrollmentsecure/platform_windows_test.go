//go:build windows

package enrollmentsecure

import (
	"bytes"
	"context"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func TestDPAPIExplicitProvisionLoadNoReplacement(t *testing.T) {
	root := filepath.Join(t.TempDir(), "keys")
	if _, err := LoadKeyProvider(root); err == nil {
		t.Fatal("missing keys auto-created")
	}
	if err := Provision(root); err != nil {
		t.Fatal(err)
	}
	provider, err := LoadKeyProvider(root)
	if err != nil {
		t.Fatal(err)
	}
	p, err := provider.ByVersion(context.Background(), PendingPhoto, "1")
	if err != nil {
		t.Fatal(err)
	}
	defer clear(p)
	template, err := provider.ByVersion(context.Background(), Template, "1")
	if err != nil {
		t.Fatal(err)
	}
	defer clear(template)
	if len(p) != 32 || len(template) != 32 || bytes.Equal(p, template) {
		t.Fatal("distinct AES-256 keys not provisioned")
	}
	if checkPrivate(root, true) != nil {
		t.Fatal("directory not owner-only")
	}
	path := filepath.Join(root, PendingPhoto+"-1.key")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(before, p) {
		t.Fatal("plaintext key on disk")
	}
	if checkPrivate(path, false) != nil {
		t.Fatal("key not owner-only")
	}
	if Provision(root) == nil {
		t.Fatal("existing keys replaced")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("reprovision changed key")
	}
	if _, err := provider.ByVersion(context.Background(), PendingPhoto, "2"); err == nil {
		t.Fatal("unknown key version accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := provider.Active(context.Background(), PendingPhoto); err == nil {
		t.Fatal("lost key hidden by cache")
	}
	if Provision(root) == nil {
		t.Fatal("partial keyset repaired automatically")
	}
}
func TestDPAPIRejectsWrongPurposeTamperingAndPlaintext(t *testing.T) {
	key := bytes.Repeat([]byte{5}, 32)
	blob, err := protectKey(key, PendingPhoto+":1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := unprotectKey(blob, PendingPhoto+":1")
	if err != nil || !bytes.Equal(got, key) {
		t.Fatal("DPAPI roundtrip failed")
	}
	clear(got)
	if _, err := unprotectKey(blob, Template+":1"); err == nil {
		t.Fatal("wrong DPAPI entropy accepted")
	}
	bad := bytes.Clone(blob)
	bad[len(bad)/2] ^= 1
	if _, err := unprotectKey(bad, PendingPhoto+":1"); err == nil {
		t.Fatal("tampered DPAPI accepted")
	}
	if _, err := unprotectKey(key, PendingPhoto+":1"); err == nil {
		t.Fatal("plaintext key accepted")
	}
	short, err := protectKey([]byte("short"), PendingPhoto+":1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unprotectKey(short, PendingPhoto+":1"); err == nil {
		t.Fatal("short DPAPI key accepted")
	}
}
func broadenDACL(t *testing.T, path string) {
	t.Helper()
	sid, err := currentSID()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + sid.String() + ")(A;;FR;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}
func TestWindowsRejectsBroadKeyAndCiphertextDACLWithoutRepair(t *testing.T) {
	root := filepath.Join(t.TempDir(), "keys")
	if err := Provision(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, PendingPhoto+"-1.key")
	broadenDACL(t, path)
	if _, err := LoadKeyProvider(root); err == nil {
		t.Fatal("broad key ACL accepted")
	}
	if checkPrivate(path, false) == nil {
		t.Fatal("loading repaired ACL instead of failing closed")
	}
	f := testStore(t)
	if err := f.Stage(testFileID, testEnvelope(t)); err != nil {
		t.Fatal(err)
	}
	if err := f.Commit(testFileID); err != nil {
		t.Fatal(err)
	}
	broadenDACL(t, filepath.Join(f.root, testFileID+".enc"))
	if _, err := f.Read(testFileID); err == nil {
		t.Fatal("broad ciphertext ACL accepted")
	}
	broadenDACL(t, f.root)
	if _, err := f.IDs(); err == nil {
		t.Fatal("broad root ACL accepted")
	}
}
func TestWindowsRejectsHardlinkedCiphertext(t *testing.T) {
	f := testStore(t)
	if err := f.Stage(testFileID, testEnvelope(t)); err != nil {
		t.Fatal(err)
	}
	if err := f.Commit(testFileID); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(f.root, testFileID+".enc"), filepath.Join(t.TempDir(), "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Read(testFileID); err == nil {
		t.Fatal("hardlink accepted")
	}
}

func TestWindowsKeyProviderRejectsRootReplacement(t *testing.T) {
	root := filepath.Join(t.TempDir(), "keys")
	if err := Provision(root); err != nil {
		t.Fatal(err)
	}
	provider, err := LoadKeyProvider(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, root+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := Provision(root); err != nil {
		t.Fatal(err)
	}
	if _, key, err := provider.Active(context.Background(), PendingPhoto); err != ErrUnavailable {
		clear(key)
		t.Fatal("provider accepted replacement key directory")
	}
	if key, err := provider.ByVersion(context.Background(), Template, "1"); err != ErrUnavailable {
		clear(key)
		t.Fatal("provider read keys from replacement directory")
	}
	replacement, err := LoadKeyProvider(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, key, err := replacement.Active(context.Background(), PendingPhoto); err != nil {
		t.Fatal(err)
	} else {
		clear(key)
	}
}
