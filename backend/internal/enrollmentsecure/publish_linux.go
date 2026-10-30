//go:build linux

package enrollmentsecure

import "golang.org/x/sys/unix"

const fileStoreSupported = true

// A single renameat2 cannot expose a link/unlink intermediate state. Kernels or
// filesystems without RENAME_NOREPLACE fail closed: never emulate it with links
// or a racy destination-existence check followed by an overwriting rename.
func moveNoReplace(from, to string) error {
	return unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE)
}
