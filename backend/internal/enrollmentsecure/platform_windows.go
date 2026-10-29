//go:build windows

package enrollmentsecure

import (
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"unsafe"
)

const persistentKeysSupported = true
const fileStoreSupported = true

var secureCrypt32 = windows.NewLazySystemDLL("crypt32.dll")
var cryptProtect = secureCrypt32.NewProc("CryptProtectData")
var cryptUnprotect = secureCrypt32.NewProc("CryptUnprotectData")

const fullFileAccess uint32 = 0x001f01ff

var secureLocalFree = windows.NewLazySystemDLL("kernel32.dll").NewProc("LocalFree")

type keyBlob struct {
	Size uint32
	Data *byte
}

func dpapi(input []byte, entropy string, protect bool) ([]byte, error) {
	if len(input) == 0 || len(input) > 16384 {
		return nil, ErrUnavailable
	}
	extra := []byte("FaceLink/FL-ENR01B/" + entropy)
	in := keyBlob{uint32(len(input)), &input[0]}
	ent := keyBlob{uint32(len(extra)), &extra[0]}
	var out keyBlob
	var ok uintptr
	if protect {
		// CurrentUser: CRYPTPROTECT_LOCAL_MACHINE is intentionally never set.
		ok, _, _ = cryptProtect.Call(uintptr(unsafe.Pointer(&in)), 0, uintptr(unsafe.Pointer(&ent)), 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	} else {
		ok, _, _ = cryptUnprotect.Call(uintptr(unsafe.Pointer(&in)), 0, uintptr(unsafe.Pointer(&ent)), 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	}
	runtime.KeepAlive(input)
	runtime.KeepAlive(extra)
	if ok == 0 {
		return nil, ErrUnavailable
	}
	defer secureLocalFree.Call(uintptr(unsafe.Pointer(out.Data)))
	plain := unsafe.Slice(out.Data, int(out.Size))
	defer clear(plain)
	if !protect && len(plain) != 32 {
		return nil, ErrUnavailable
	}
	return append([]byte(nil), plain...), nil
}
func protectKey(key []byte, entropy string) ([]byte, error)   { return dpapi(key, entropy, true) }
func unprotectKey(key []byte, entropy string) ([]byte, error) { return dpapi(key, entropy, false) }
func currentSID() (*windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, ErrUnavailable
	}
	return user.User.Sid, nil
}
func protectPath(path string, directory bool) error {
	sid, err := currentSID()
	if err != nil {
		return err
	}
	flags := ""
	if directory {
		flags = "OICI"
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;" + flags + ";FA;;;" + sid.String() + ")")
	if err != nil {
		return ErrUnavailable
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return ErrUnavailable
	}
	// Only DACL information is written. No owner/group/SACL, no elevation or
	// SeSecurityPrivilege. Unlike the OTP helper, SYSTEM is not granted access.
	if windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil) != nil {
		return ErrUnavailable
	}
	return checkPrivate(path, directory)
}
func checkPrivate(path string, directory bool) error {
	sid, err := currentSID()
	if err != nil {
		return err
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return ErrUnavailable
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return ErrUnavailable
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil || acl.AceCount != 1 {
		return ErrUnavailable
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if windows.GetAce(acl, 0, &ace) != nil || ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&windows.INHERITED_ACE != 0 || uint32(ace.Mask)&fullFileAccess != fullFileAccess {
		return ErrUnavailable
	}
	actual := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !actual.IsValid() || !windows.EqualSid(actual, sid) {
		return ErrUnavailable
	}
	expectedFlags := uint8(0)
	if directory {
		expectedFlags = windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE
	}
	if ace.Header.AceFlags != expectedFlags {
		return ErrUnavailable
	}
	if !directory {
		p, e := windows.UTF16PtrFromString(path)
		if e != nil {
			return ErrUnavailable
		}
		handle, e := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if e != nil {
			return ErrUnavailable
		}
		var info windows.ByHandleFileInformation
		e = windows.GetFileInformationByHandle(handle, &info)
		windows.CloseHandle(handle)
		if e != nil || info.NumberOfLinks != 1 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return ErrUnavailable
		}
	}
	return nil
}
func unsafePlatformPath(path string) bool {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return true
	}
	attrs, err := windows.GetFileAttributes(p)
	return err != nil || attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
func moveNoReplace(from, to string) error {
	a, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	b, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	// No MOVEFILE_REPLACE_EXISTING. Same-directory rename, durable write-through.
	return windows.MoveFileEx(a, b, windows.MOVEFILE_WRITE_THROUGH)
}

// File.Sync uses FlushFileBuffers; namespace moves use MOVEFILE_WRITE_THROUGH.
// Windows does not support portable directory fsync; interrupted deletions are
// handled idempotently by reconciliation rather than claiming forensic erasure.
func syncDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || unsafePlatformPath(path) {
		return ErrUnavailable
	}
	return nil
}
func openNoFollow(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(handle, &info); err != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.NumberOfLinks != 1 {
		windows.CloseHandle(handle)
		return nil, ErrUnavailable
	}
	return os.NewFile(uintptr(handle), path), nil
}

// A crash immediately after CREATE_NEW leaves an empty but already private
// orphan. No separate DACL-repair step is needed for IDs/Delete to recover it.
func createPrivateFile(path string) (*os.File, error) {
	sid, err := currentSID()
	if err != nil {
		return nil, err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + sid.String() + ")")
	if err != nil {
		return nil, ErrUnavailable
	}
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	handle, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, &sa, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	runtime.KeepAlive(sd)
	if err != nil {
		return nil, ErrUnavailable
	}
	file := os.NewFile(uintptr(handle), path)
	if checkPrivate(path, false) != nil {
		file.Close()
		return nil, ErrUnavailable
	}
	return file, nil
}

func openDirectoryNoFollow(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	handle, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, ErrUnavailable
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		windows.CloseHandle(handle)
		return nil, ErrUnavailable
	}
	return os.NewFile(uintptr(handle), path), nil
}
