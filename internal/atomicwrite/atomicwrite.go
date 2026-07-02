// Package atomicwrite writes a file such that a concurrent reader — or a
// crash — never observes a half-written result: the bytes are written to a
// temp file in the *same directory* as the target, flushed, and then
// atomically moved into place (implementation-plan.md §3, "Atomic writes").
//
// The move is platform-specific: os.Rename on unix (atomic same-filesystem
// replace), and MoveFileEx(REPLACE_EXISTING|WRITE_THROUGH) on Windows,
// whose plain os.Rename cannot replace an existing file. google/renameio —
// the obvious off-the-shelf choice — exports nothing on Windows and is
// therefore disqualified here (plan §3).
//
// Guarantees, stated precisely:
//
//   - Atomicity against PROCESS DEATH, on every platform: at any kill
//     point the target path holds either the complete old content or the
//     complete new content, never a mixture. This is the property the
//     crash-injection tests exercise.
//   - Durability against POWER LOSS: on unix, the file data is fsynced
//     before the rename and the parent directory is fsynced after it, so
//     both the content and the new directory entry are durable when
//     WriteFile returns. On Windows, MoveFileEx WRITE_THROUGH "does not
//     return until the file is actually moved on the disk", but its
//     documented flush guarantee covers only moves performed as
//     copy+delete (cross-volume) — directory-entry durability for our
//     same-volume rename is not explicitly documented; see
//     rename_windows.go. The backstop there is the domain's recovery
//     story: re-run + regen self-heals.
//
// This is the write primitive for every mutating command's file
// operations (the ADR files, constitution.md, the manifest): a torn write
// is the one failure the "log is truth, regen self-heals" recovery story
// cannot paper over, so it is designed out at the syscall level.
package atomicwrite

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile atomically replaces path with data. The temp file is created
// in filepath.Dir(path) so the final move is a same-filesystem rename (a
// cross-device rename is not atomic and would fall back to copy). On any
// error before the move, the temp file is removed and path is left
// untouched.
func WriteFile(path string, data []byte, perm os.FileMode) (err error) {
	dir := filepath.Dir(path)

	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("atomicwrite: create temp file in %s: %w", dir, err)
	}
	tmp := f.Name()

	// Remove the temp file unless we hand it off via a successful move.
	// Guards every early-return error path below.
	defer func() {
		if tmp != "" {
			_ = os.Remove(tmp)
		}
	}()

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("atomicwrite: write temp file: %w", err)
	}
	// Flush to disk before the rename so the moved-into-place file has
	// durable contents even if the machine loses power right after.
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("atomicwrite: sync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("atomicwrite: close temp file: %w", err)
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return fmt.Errorf("atomicwrite: chmod temp file: %w", err)
	}

	if err := replace(tmp, path); err != nil {
		return fmt.Errorf("atomicwrite: replace %s: %w", path, err)
	}
	tmp = "" // handed off; the deferred cleanup must not delete it now
	return nil
}
