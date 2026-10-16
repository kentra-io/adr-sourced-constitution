package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	root "github.com/kentra-io/adr-sourced-constitution"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
	"github.com/kentra-io/adr-sourced-constitution/internal/config"
)

// Legacy managed-block interiors: what CLI versions before 0.4.0 wrote
// into CLAUDE.md (an `@import`, which loaded the constitution into every
// session) and AGENTS.md (a textual pointer). 0.4.0 writes no blocks; these
// are kept only so removeLegacyBlocks can recognise an unedited old block.
const (
	legacyClaudeInterior = "@constitution/constitution.md"
	legacyAgentsInterior = "Before planning, read `constitution/constitution.md`; it is this project's governing constitution and takes precedence over inferred conventions."
)

// legacyBlockFiles are the fixed files an older CLI may have written a
// managed block into. Removal checks both regardless of the config: the
// targets that selected them no longer exist in 0.4.0.
var legacyBlockFiles = []struct {
	rel      string // repo-root-relative, slash form
	interior string
}{
	{rel: "CLAUDE.md", interior: legacyClaudeInterior},
	{rel: "AGENTS.md", interior: legacyAgentsInterior},
}

// Mode selects the drift policy Refresh applies (plan §2.2, §6):
//   - ModeInit: a drifted skill file is rewritten only after --force or an
//     interactive confirm; otherwise Refresh refuses (returns an error). A
//     hand-edited legacy pointer block is removed only after --force or an
//     interactive confirm; otherwise Refresh warns and leaves it.
//   - ModeWarn: a drifted target is never rewritten — Refresh warns and
//     leaves it, so drift in a user file can never block the mutating verbs
//     that auto-run regen.
type Mode int

// Refresh modes (see Mode).
const (
	ModeInit Mode = iota
	ModeWarn
)

// Options configures a Refresh pass.
type Options struct {
	Root    string
	Cfg     *config.Config
	Mode    Mode
	Force   bool                              // ModeInit: overwrite drift without prompting
	Confirm func(prompt string) (bool, error) // ModeInit interactive confirm; nil ⇒ never confirm
	Stdout  io.Writer                         // progress ("wrote ...")
	Stderr  io.Writer                         // warnings
}

// Refresh removes the managed pointer blocks older CLI versions wrote
// (removeLegacyBlocks) and writes/updates the fanned-out skill copies named
// by o.Cfg, drift-protected via constitution/.state. It is the shared engine
// behind `init` (ModeInit) and `regen` (ModeWarn). A repo that manages
// nothing and has no .state stays that way: no .state file is created.
func Refresh(o Options) error {
	st := loadStateOrEmpty(o.Root, o.Stderr)

	if err := o.removeLegacyBlocks(st); err != nil {
		return err
	}

	items, err := skillItems(o.Cfg)
	if err != nil {
		return err
	}
	for _, it := range items {
		if err := o.refreshFile(st, it); err != nil {
			return err
		}
	}

	// Don't materialize an empty .state in a repo that manages nothing —
	// that keeps non-integrated repos' trees byte-clean. Once anything is
	// managed (or a .state already exists), persist.
	if st.empty() && !stateExists(o.Root) {
		return nil
	}
	return st.Save(o.Root)
}

// loadStateOrEmpty loads constitution/.state, degrading to a fresh empty
// state (with a prominent stderr warning) when the file is corrupt or its
// schemaVersion is unrecognized. This upholds the binding invariant that
// auto-regen — and the mutating verbs that trigger it — can never be blocked
// by the state of CLI-owned bookkeeping (plan §2.2, §6): a hard error here
// would propagate out of Refresh and make `adr new` exit non-zero AFTER the
// ADR already landed, lying via the exit code. Degrading is safe because
// drift detection is content-hash based: against an empty state a matching
// file is still a no-op, and a drifted one still triggers the
// refuse/--force path (init) or the warn path (regen). The .state is rebuilt
// on the next successful write.
func loadStateOrEmpty(root string, stderr io.Writer) *State {
	st, err := LoadState(root)
	if err != nil {
		warnf(stderr, "warning: %s; ignoring it and proceeding with an empty drift state (managed files will be reconciled on the next `constitution init`)", err.Error())
		return newState()
	}
	return st
}

// removeLegacyBlocks removes the always-loaded constitution block that CLI
// versions before 0.4.0 wrote into CLAUDE.md and AGENTS.md. It never
// returns an error for a file it decides to leave alone — it warns instead —
// so a stale block can never block a mutating verb's auto-regen.
//
// Regular files are handled first and symlinks last: a symlink is never
// rewritten (an atomic rename would replace the link with a plain file), and
// checking it after its target was cleaned (the common CLAUDE.md ->
// AGENTS.md setup) avoids a spurious warning.
func (o Options) removeLegacyBlocks(st *State) error {
	var links []string
	removed := map[string]string{} // absolute path -> rel, of files deleted below
	for _, f := range legacyBlockFiles {
		stored, hasStored := st.get(f.rel)
		// No managed blocks exist any more: never leave stale bookkeeping.
		st.remove(f.rel)

		fpath := filepath.Join(o.Root, filepath.FromSlash(f.rel))
		info, err := os.Lstat(fpath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			links = append(links, f.rel)
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		gone, err := o.removeLegacyBlock(fpath, f.rel, f.interior, stored, hasStored)
		if err != nil {
			return err
		}
		if gone {
			removed[fpath] = f.rel
		}
	}

	for _, rel := range links {
		// Follows the link; a dangling one reads as absent.
		lpath := filepath.Join(o.Root, filepath.FromSlash(rel))
		content, existed, err := readFileIfExists(lpath)
		if err != nil {
			warnf(o.Stderr, "constitution: cannot read %s through its symlink (%s); leaving it untouched", rel, err.Error())
			continue
		}
		if !existed {
			if dest, rerr := os.Readlink(lpath); rerr == nil {
				if !filepath.IsAbs(dest) {
					dest = filepath.Join(filepath.Dir(lpath), dest)
				}
				if target, ok := removed[filepath.Clean(dest)]; ok {
					warnf(o.Stderr, "constitution: %s is a symlink to %s, which held only the always-loaded constitution block and was removed; delete the link or point it elsewhere", rel, target)
				}
			}
			continue
		}
		if found, _, _, _, lerr := LocateBlock(content); found || lerr != nil {
			warnf(o.Stderr, "constitution: %s is a symlink and still holds the always-loaded constitution block; leaving it untouched — remove the block by hand from the file it points to", rel)
		}
	}
	return nil
}

// removeLegacyBlock removes the managed block from one regular file when it
// is safe to: the interior is exactly what an older CLI wrote, or matches
// the hash .state recorded for it. A hand-edited interior is removed only by
// `init` under --force or an interactive yes; otherwise, and always for a
// malformed marker pair, the file is left untouched with a warning.
func (o Options) removeLegacyBlock(fpath, rel, legacyInterior, stored string, hasStored bool) (removed bool, err error) {
	content, _, err := readFileIfExists(fpath)
	if err != nil {
		return false, err
	}

	found, _, _, interior, lerr := LocateBlock(content)
	if lerr != nil {
		var me *MarkerError
		if errors.As(lerr, &me) {
			me.Path = rel
		}
		warnf(o.Stderr, "constitution: %s; leaving it untouched — remove the always-loaded constitution block by hand", lerr.Error())
		return false, nil
	}
	if !found {
		return false, nil
	}

	known := interior == legacyInterior || (hasStored && stored == hashContent([]byte(interior)))
	if !known {
		ok, err := o.confirmDriftedRemoval(rel)
		if err != nil {
			return false, err
		}
		if !ok {
			warnf(o.Stderr, "constitution: the always-loaded constitution block in %s was edited by hand; leaving it — remove it by hand or run `constitution init --force`", rel)
			return false, nil
		}
	}

	out, _, err := RemoveBlock(content)
	if err != nil {
		return false, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		if err := os.Remove(fpath); err != nil {
			return false, err
		}
		o.progressf("constitution: removed %s, which held only the always-loaded constitution block", rel)
		return true, nil
	}
	if err := writeAtomic(fpath, out); err != nil {
		return false, err
	}
	o.progressf("constitution: removed the always-loaded constitution block from %s", rel)
	return false, nil
}

// confirmDriftedRemoval decides whether a hand-edited legacy block may be
// removed: only in ModeInit, under --force or an interactive yes.
func (o Options) confirmDriftedRemoval(rel string) (bool, error) {
	if o.Mode != ModeInit {
		return false, nil
	}
	if o.Force {
		return true, nil
	}
	if o.Confirm == nil {
		return false, nil
	}
	return o.Confirm(fmt.Sprintf("the always-loaded constitution block in %s was edited by hand; remove it anyway?", rel))
}

type skillItem struct {
	rel     string // repo-root-relative, slash form
	content []byte
}

func (o Options) refreshFile(st *State, it skillItem) error {
	fpath := filepath.Join(o.Root, filepath.FromSlash(it.rel))
	content, existed, err := readFileIfExists(fpath)
	if err != nil {
		return err
	}

	desiredHash := hashContent(it.content)
	write := func() error {
		if err := writeAtomic(fpath, it.content); err != nil {
			return err
		}
		st.set(it.rel, desiredHash)
		o.progressf("wrote %s", it.rel)
		return nil
	}

	switch {
	case existed && bytes.Equal(content, it.content):
		st.set(it.rel, desiredHash)
		return nil
	case !existed:
		return write()
	}

	stored, hasStored := st.get(it.rel)
	if hasStored && stored == hashContent(content) {
		return write()
	}
	return o.handleDrift(st, it.rel, write, stored, hasStored)
}

// handleDrift applies the mode's policy to a target whose on-disk content
// diverged from what the CLI last wrote (plan §2.2, §6).
func (o Options) handleDrift(st *State, rel string, write func() error, stored string, hasStored bool) error {
	if o.Mode == ModeWarn {
		warnf(o.Stderr, "regen: %s drifted from what `constitution init` last wrote; leaving it untouched (run `constitution init` to reconcile)", rel)
		// Preserve the prior recorded hash so a later `init` still sees the
		// drift; if we never wrote this target, record nothing for it.
		if hasStored {
			st.set(rel, stored)
		}
		return nil
	}
	if o.Force {
		return write()
	}
	if o.Confirm != nil {
		ok, err := o.Confirm(fmt.Sprintf("%s drifted from what `constitution init` last wrote; overwrite it with the bundled copy?", rel))
		if err != nil {
			return err
		}
		if ok {
			return write()
		}
	}
	return fmt.Errorf(
		"%s drifted from what `constitution init` last wrote; refusing to overwrite it. Re-run with --force to overwrite the skill file with the bundled copy, or reconcile the file by hand",
		rel,
	)
}

func skillItems(cfg *config.Config) ([]skillItem, error) {
	if len(cfg.Skills.Trees) == 0 {
		return nil, nil
	}
	names, err := skillNames()
	if err != nil {
		return nil, err
	}
	var items []skillItem
	for _, tree := range cfg.Skills.Trees {
		dir, ok := treeDir(tree)
		if !ok {
			continue
		}
		for _, name := range names {
			content, err := skillContent(name)
			if err != nil {
				return nil, err
			}
			items = append(items, skillItem{
				rel:     path.Join(dir, "skills", name, "SKILL.md"),
				content: content,
			})
		}
	}
	return items, nil
}

// treeDir maps a skills-tree key to its on-disk directory (plan §6).
func treeDir(tree string) (string, bool) {
	switch tree {
	case config.SkillTreeClaude:
		return ".claude", true
	case config.SkillTreeAgents:
		return ".agents", true
	case config.SkillTreeCursor:
		return ".cursor", true
	}
	return "", false
}

// bootstrapOnlySkills are embedded so the marketplace plugin can ship them,
// but are never fanned into a governed repo: they exist to *create* a
// constitution, not to operate one. Adopters get these from the plugin (user
// scope); a governed repo only carries the ongoing-governance skills.
var bootstrapOnlySkills = map[string]bool{
	"constitution-init": true,
}

// skillNames returns the embedded skills' directory names, in ReadDir order
// (sorted), so fan-out is deterministic. Bootstrap-only skills (see
// bootstrapOnlySkills) are excluded — they are delivered by the plugin, not
// copied into the governed repo.
func skillNames() ([]string, error) {
	entries, err := fs.ReadDir(root.SkillsFS, "skills")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !bootstrapOnlySkills[e.Name()] {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// skillContent returns the embedded SKILL.md bytes for a skill.
func skillContent(name string) ([]byte, error) {
	return root.SkillsFS.ReadFile(path.Join("skills", name, "SKILL.md"))
}

func readFileIfExists(fpath string) (content []byte, existed bool, err error) {
	data, err := os.ReadFile(fpath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

func writeAtomic(fpath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
		return err
	}
	return atomicwrite.WriteFile(fpath, data, 0o644)
}

func stateExists(repoRoot string) bool {
	_, err := os.Stat(statePath(repoRoot))
	return err == nil
}

func warnf(w io.Writer, format string, a ...any) {
	if w != nil {
		// Best-effort diagnostic; a failed warning write must not derail a
		// refresh (matching the consent gate's `_, _ =` convention).
		_, _ = fmt.Fprintf(w, format+"\n", a...)
	}
}

func (o Options) progressf(format string, a ...any) {
	if o.Stdout != nil {
		_, _ = fmt.Fprintf(o.Stdout, format+"\n", a...)
	}
}
