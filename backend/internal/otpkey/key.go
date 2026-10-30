// Package otpkey memuat key HMAC OTP dari penyimpanan lokal terlindungi.
package otpkey

import (
	"errors"
	"os"
	"path/filepath"
)

// Load menolak key dalam repository. Windows memakai DPAPI CurrentUser;
// Unix memakai berkas base64 dengan permission owner-only.
func Load(path string) ([]byte, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.New("OTP key: jalur tidak sah")
	}
	p, err = filepath.EvalSymlinks(p)
	if err != nil {
		return nil, errors.New("OTP key: berkas tidak tersedia")
	}
	for dir := filepath.Dir(p); ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return nil, errors.New("OTP key wajib di luar repository")
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	info, err := os.Stat(p)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16384 {
		return nil, errors.New("OTP key: berkas tidak sah")
	}
	return loadProtected(p, info)
}
