package rename

import "os"

// Atomic provides an atomic file rename.  newpath is replaced if it
// already exists.
func Atomic(src, dst string) error {
	return os.Rename(src, dst)
}
