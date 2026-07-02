//go:build windows

package atomicwrite

import "golang.org/x/sys/windows"

// replace atomically moves tmp onto dst on Windows. Plain os.Rename
// (MoveFile) fails if dst already exists, so a status transition or a regen
// over an existing file would break; MoveFileEx with REPLACE_EXISTING
// performs the atomic replace, and WRITE_THROUGH flushes the rename through
// to disk so a crash immediately after cannot lose it (plan §3).
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
