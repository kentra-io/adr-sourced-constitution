//go:build !windows

package atomicwrite

import "os"

// replace atomically moves tmp onto dst. On unix, os.Rename is a single
// rename(2) syscall: an atomic same-filesystem replace of an existing file.
func replace(tmp, dst string) error {
	return os.Rename(tmp, dst)
}
