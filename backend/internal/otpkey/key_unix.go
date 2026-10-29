//go:build !windows

package otpkey

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"
)

func loadProtected(path string, info os.FileInfo) ([]byte, error) {
	if info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("OTP key: permission wajib owner-only (chmod 600)")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("OTP key: tidak dapat dibaca")
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
	clear(b)
	if err != nil || len(key) != 32 {
		return nil, errors.New("OTP key: base64 32 byte wajib")
	}
	return key, nil
}
