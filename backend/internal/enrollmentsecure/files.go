package enrollmentsecure

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// FileStore contains ciphertext only. Root must be outside any Git repository.
// A .tmp left by a crash is included in IDs and removed by Delete, so domain
// reconciliation can remove all orphan states without ever needing a key.
type FileStore struct {
	root     string
	identity os.FileInfo
	mu       sync.Mutex
}

func safeID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func safePath(path string) (string, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return "", ErrUnavailable
	}
	p = filepath.Clean(p)
	for dir := p; ; dir = filepath.Dir(dir) {
		info, e := os.Lstat(dir)
		if e == nil {
			if info.Mode()&os.ModeSymlink != 0 || unsafePlatformPath(dir) {
				return "", ErrUnavailable
			}
		} else if !errors.Is(e, os.ErrNotExist) {
			return "", ErrUnavailable
		}
		if e != nil || info.IsDir() {
			if _, gitErr := os.Lstat(filepath.Join(dir, ".git")); gitErr == nil || !errors.Is(gitErr, os.ErrNotExist) {
				return "", ErrUnavailable
			}
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return p, nil
}

func privateRoot(path string, create bool) (string, error) {
	p, err := safePath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(p)
	if errors.Is(err, os.ErrNotExist) && create {
		if os.Mkdir(p, 0700) != nil {
			return "", ErrUnavailable
		}
		if protectPath(p, true) != nil {
			return "", ErrUnavailable
		}
		info, err = os.Lstat(p)
	}
	if err != nil || !info.IsDir() || checkPrivate(p, true) != nil {
		return "", ErrUnavailable
	}
	return p, nil
}

func NewFileStore(root string) (*FileStore, error) {
	if !fileStoreSupported {
		return nil, ErrUnavailable
	}
	p, err := privateRoot(root, true)
	if err != nil {
		return nil, err
	}
	info, err := rootIdentity(p)
	if err != nil {
		return nil, ErrUnavailable
	}
	return &FileStore{root: p, identity: info}, nil
}
func (f *FileStore) ready() error {
	if f == nil {
		return ErrUnavailable
	}
	p, err := privateRoot(f.root, false)
	if err != nil {
		return err
	}
	info, err := rootIdentity(p)
	if err != nil || !os.SameFile(info, f.identity) {
		return ErrUnavailable
	}
	return nil
}

// Windows pathname-derived FileInfo defers loading its file ID until SameFile.
// Capture through an opened directory handle instead, so a later replacement
// cannot retroactively change the identity recorded by a constructor.
func rootIdentity(path string) (os.FileInfo, error) {
	if _, err := safePath(path); err != nil {
		return nil, err
	}
	directory, err := openDirectoryNoFollow(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	info, statErr := directory.Stat()
	closeErr := directory.Close()
	if statErr != nil || closeErr != nil || !info.IsDir() || unsafePlatformPath(path) || checkPrivate(path, true) != nil {
		return nil, ErrUnavailable
	}
	return info, nil
}
func (f *FileStore) checkFile(path string, missing bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && missing {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() || unsafePlatformPath(path) || checkPrivate(path, false) != nil {
		return ErrUnavailable
	}
	return nil
}

const maxEnvelopeBytes = 2 * MaxImageBytes

func (f *FileStore) Stage(id string, e Envelope) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !safeID(id) || !validEnvelope(e) {
		return ErrIntegrity
	}
	if err := f.ready(); err != nil {
		return err
	}
	for _, suffix := range []string{".tmp", ".stage", ".enc"} {
		if _, err := os.Lstat(filepath.Join(f.root, id+suffix)); !errors.Is(err, os.ErrNotExist) {
			return ErrUnavailable
		}
	}
	encoded, err := json.Marshal(e)
	if err != nil || len(encoded) > maxEnvelopeBytes {
		return ErrIntegrity
	}
	path := filepath.Join(f.root, id+".tmp")
	file, err := createPrivateFile(path)
	if err != nil {
		return ErrUnavailable
	}
	_, err = file.Write(encoded)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return ErrUnavailable
	}
	if err = moveNoReplace(path, filepath.Join(f.root, id+".stage")); err != nil {
		return ErrUnavailable
	}
	return syncDirectory(f.root)
}
func (f *FileStore) Commit(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !safeID(id) {
		return ErrIntegrity
	}
	if err := f.ready(); err != nil {
		return err
	}
	from, to := filepath.Join(f.root, id+".stage"), filepath.Join(f.root, id+".enc")
	if err := f.checkFile(from, false); err != nil {
		return err
	}
	// Never promote a partial or malformed staged envelope.
	if _, err := readEnvelope(from); err != nil {
		return err
	}
	if moveNoReplace(from, to) != nil {
		return ErrUnavailable
	}
	return syncDirectory(f.root)
}
func readEnvelope(path string) (Envelope, error) {
	file, err := openPrivateRead(path)
	if err != nil {
		return Envelope{}, ErrUnavailable
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > maxEnvelopeBytes || !info.Mode().IsRegular() {
		return Envelope{}, ErrIntegrity
	}
	b, err := io.ReadAll(io.LimitReader(file, maxEnvelopeBytes+1))
	if err != nil || len(b) > maxEnvelopeBytes {
		return Envelope{}, ErrIntegrity
	}
	var e Envelope
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if dec.Decode(&e) != nil || dec.Decode(new(any)) != io.EOF || !validEnvelope(e) {
		return Envelope{}, ErrIntegrity
	}
	return e, nil
}
func (f *FileStore) Read(id string) (Envelope, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !safeID(id) {
		return Envelope{}, ErrIntegrity
	}
	if err := f.ready(); err != nil {
		return Envelope{}, err
	}
	path := filepath.Join(f.root, id+".enc")
	if err := f.checkFile(path, false); err != nil {
		return Envelope{}, err
	}
	return readEnvelope(path)
}
func (f *FileStore) Delete(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !safeID(id) {
		return ErrIntegrity
	}
	if err := f.ready(); err != nil {
		return err
	}
	// Validate every entry before deleting anything; never follow a reparse point.
	for _, suffix := range []string{".tmp", ".stage", ".enc"} {
		if err := f.checkFile(filepath.Join(f.root, id+suffix), true); err != nil {
			return err
		}
	}
	for _, suffix := range []string{".tmp", ".stage", ".enc"} {
		if err := os.Remove(filepath.Join(f.root, id+suffix)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return ErrUnavailable
		}
	}
	return syncDirectory(f.root)
}
func (f *FileStore) IDs() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ready(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(f.root)
	if err != nil {
		return nil, ErrUnavailable
	}
	ids := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		ext := filepath.Ext(name)
		id := strings.TrimSuffix(name, ext)
		if !safeID(id) || (ext != ".enc" && ext != ".stage" && ext != ".tmp") {
			return nil, ErrIntegrity
		}
		if err := f.checkFile(filepath.Join(f.root, name), false); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}
func openPrivateRead(path string) (*os.File, error) {
	if _, err := safePath(path); err != nil {
		return nil, err
	}
	file, err := openNoFollow(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	info, err := file.Stat()
	actual, e := os.Lstat(path)
	if err != nil || e != nil || !info.Mode().IsRegular() || !os.SameFile(info, actual) || checkPrivate(path, false) != nil || unsafePlatformPath(path) {
		file.Close()
		return nil, ErrUnavailable
	}
	return file, nil
}
