//go:build windows

package atomicwrite

import "golang.org/x/sys/windows"

// replace atomically moves tmp onto dst on Windows. Plain os.Rename
// (MoveFile) fails if dst already exists, so a status transition or a regen
// over an existing file would break; MoveFileEx with REPLACE_EXISTING
// performs the atomic replace (plan §3).
//
// Durability, stated precisely per the MoveFileExW documentation:
// WRITE_THROUGH means "the function does not return until the file is
// actually moved on the disk", but its explicit flush-to-disk guarantee is
// scoped to moves "performed as a copy and delete operation" — i.e.
// cross-volume moves, which never happen here (the temp file is in the
// same directory). For our same-volume rename, directory-entry durability
// against POWER LOSS is therefore not explicitly documented. Atomicity
// against process death is unaffected (the rename either happened or it
// didn't); if a power loss does roll back a directory entry, the backstop
// is the domain's recovery story — re-run the command and `regen`
// self-heals, since every file this package writes is either an
// append-only log record or a pure projection of the log.
func replace(tmp, dst string) error {
	from, err := windows.UTF16PtrFromString(tmp)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
