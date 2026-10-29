//go:build !windows

package enrollmentsecure

import (
	"os"
	"syscall"
)

// Persistent keys are Windows DPAPI CurrentUser only; tests and synthetic
// callers may explicitly supply an in-memory KeyProvider on other platforms.
const persistentKeysSupported = false

func protectKey([]byte, string) ([]byte, error)   { return nil, ErrUnavailable }
func unprotectKey([]byte, string) ([]byte, error) { return nil, ErrUnavailable }
func protectPath(path string, directory bool) error {
	mode := os.FileMode(0600)
	if directory {
		mode = 0700
	}
	if os.Chmod(path, mode) != nil {
		return ErrUnavailable
	}
	return checkPrivate(path, directory)
}
func checkPrivate(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return ErrUnavailable
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) || (!directory && stat.Nlink != 1) {
		return ErrUnavailable
	}
	return nil
}
func unsafePlatformPath(path string) bool {
	info, err := os.Lstat(path)
	return err != nil || info.Mode()&os.ModeSymlink != 0
}
func createPrivateFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
}
func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return ErrUnavailable
	}
	defer f.Close()
	if f.Sync() != nil {
		return ErrUnavailable
	}
	return nil
}
func openNoFollow(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func openDirectoryNoFollow(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
