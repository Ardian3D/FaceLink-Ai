package enrollmentsecure

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
)

type diskKeys struct {
	root     string
	identity os.FileInfo
}

// Provision is an explicit operator action. It refuses any existing key, even
// after interrupted provisioning. Recovery/rekeying is never automatic.
// Windows is mandatory for persistent biometric keys; other platforms fail closed.
func Provision(root string) error {
	if !persistentKeysSupported {
		return ErrUnavailable
	}
	p, err := privateRoot(root, true)
	if err != nil {
		return err
	}
	for _, purpose := range []string{PendingPhoto, Template} {
		if _, err = os.Lstat(filepath.Join(p, purpose+"-1.key")); !os.IsNotExist(err) {
			return ErrUnavailable
		}
	}
	previous := make([]byte, 32)
	defer clear(previous)
	for i, purpose := range []string{PendingPhoto, Template} {
		key := make([]byte, 32)
		if _, err = io.ReadFull(rand.Reader, key); err != nil {
			clear(key)
			return ErrUnavailable
		}
		if i > 0 && bytes.Equal(key, previous) {
			clear(key)
			return ErrUnavailable
		}
		copy(previous, key)
		protected, e := protectKey(key, purpose+":1")
		clear(key)
		if e != nil {
			return ErrUnavailable
		}
		file, e := createPrivateFile(filepath.Join(p, purpose+"-1.key"))
		if e != nil {
			return ErrUnavailable
		}
		_, e = file.Write(protected)
		if e == nil {
			e = file.Sync()
		}
		closeErr := file.Close()
		if e != nil || closeErr != nil {
			return ErrUnavailable
		}
	}
	return syncDirectory(p)
}

// LoadKeyProvider never creates or repairs anything. Active reads DPAPI files on
// every call, so lost/inaccessible keys cannot be hidden by an in-memory cache.
func LoadKeyProvider(root string) (KeyProvider, error) {
	if !persistentKeysSupported {
		return nil, ErrUnavailable
	}
	p, err := privateRoot(root, false)
	if err != nil {
		return nil, err
	}
	info, err := rootIdentity(p)
	if err != nil {
		return nil, ErrUnavailable
	}
	provider := &diskKeys{root: p, identity: info}
	photo, err := provider.ByVersion(context.Background(), PendingPhoto, "1")
	defer clear(photo)
	if err != nil {
		return nil, err
	}
	template, err := provider.ByVersion(context.Background(), Template, "1")
	defer clear(template)
	if err != nil || bytes.Equal(photo, template) {
		return nil, ErrUnavailable
	}
	return provider, nil
}
func (k *diskKeys) Active(ctx context.Context, purpose string) (string, []byte, error) {
	key, err := k.ByVersion(ctx, purpose, "1")
	if err != nil {
		return "", nil, err
	}
	return "1", key, nil
}
func (k *diskKeys) ByVersion(ctx context.Context, purpose, version string) ([]byte, error) {
	if ctx.Err() != nil || version != "1" || (purpose != PendingPhoto && purpose != Template) {
		return nil, ErrUnavailable
	}
	p, err := privateRoot(k.root, false)
	if err != nil {
		return nil, err
	}
	info, err := rootIdentity(p)
	if err != nil || !os.SameFile(info, k.identity) {
		return nil, ErrUnavailable
	}
	path := filepath.Join(p, purpose+"-1.key")
	info, err = os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 16384 || unsafePlatformPath(path) || checkPrivate(path, false) != nil {
		return nil, ErrUnavailable
	}
	file, err := openPrivateRead(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer file.Close()
	protected, err := io.ReadAll(io.LimitReader(file, 16385))
	if err != nil || len(protected) > 16384 {
		return nil, ErrUnavailable
	}
	return unprotectKey(protected, purpose+":"+version)
}
