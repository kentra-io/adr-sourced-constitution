//go:build !windows

package atomicwrite

import (
	"os"
	"path/filepath"
)

// replace atomically moves tmp onto dst. On unix, os.Rename is a single
// rename(2) syscall: an atomic same-filesystem replace of an existing file.
// The parent directory is fsynced afterwards so the new directory entry is
// durable — without it, a power loss shortly after the rename can roll the
// directory back to the pre-rename entry even though the file data was
// synced (the file-data fsync happens in WriteFile, before the rename).
func replace(tmp, dst string) error {
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(dst))
	if err != nil {
		return err
	}
	// Close error on a read-only directory handle is not actionable; the
	// durability-relevant call is Sync, whose error is returned.
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}
