//go:build !windows && !linux

package enrollmentsecure

// Only Windows write-through no-replace rename and Linux renameat2 currently
// implement the required publication contract. Do not create store directories
// or silently degrade to a non-atomic link/unlink sequence elsewhere.
const fileStoreSupported = false

func moveNoReplace(string, string) error { return ErrUnavailable }
