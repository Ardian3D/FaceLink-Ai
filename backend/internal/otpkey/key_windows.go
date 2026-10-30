//go:build windows

package otpkey

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"unsafe"
)

type dataBlob struct {
	Size uint32
	Data *byte
}

var crypt32 = windows.NewLazySystemDLL("crypt32.dll")
var unprotect = crypt32.NewProc("CryptUnprotectData")
var localFree = windows.NewLazySystemDLL("kernel32.dll").NewProc("LocalFree")

func loadProtected(path string, info os.FileInfo) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return nil, errors.New("OTP key: tidak dapat dibaca")
	}
	in := dataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out dataBlob
	// CRYPTPROTECT_UI_FORBIDDEN: proses server tidak boleh meminta UI interaktif.
	ok, _, _ := unprotect.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(b)
	if ok == 0 {
		return nil, errors.New("OTP key: DPAPI CurrentUser tidak dapat membuka key; jangan membuat key pengganti otomatis")
	}
	defer localFree.Call(uintptr(unsafe.Pointer(out.Data)))
	if out.Size != 32 {
		return nil, errors.New("OTP key: wajib 32 byte")
	}
	plain := unsafe.Slice(out.Data, int(out.Size))
	key := append([]byte(nil), plain...)
	clear(plain)
	return key, nil
}
