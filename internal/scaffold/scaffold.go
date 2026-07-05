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

// Managed-block interiors (plan §2.1/§5). CLAUDE.md gets Claude Code's
// direct `@import` (real inlining, resolved relative to the repo root);
// AGENTS.md gets a short textual pointer, the regime where cross-tool
// pointers are actually followed.
const (
	claudeInterior = "@constitution/constitution.md"
	agentsInterior = "Before planning, read `constitution/constitution.md`; it is this project's governing constitution and takes precedence over inferred conventions."
)

// Mode selects the drift policy Refresh applies (plan §2.2, §6):
//   - ModeInit: a drifted target is rewritten only after --force or an
//     interactive confirm; otherwise Refresh refuses (returns an error).
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

// Refresh writes/updates the managed pointer blocks and fanned-out skill
// copies named by o.Cfg, drift-protected via constitution/.state. It is the
// shared engine behind `init` (ModeInit) and `regen` (ModeWarn). It manages
// only what the config selects: a config with no targets and no skill trees
// (the pre-M4 shape) is a no-op that never creates a .state file.
func Refresh(o Options) error {
	st := loadStateOrEmpty(o.Root, o.Stderr)

	for _, t := range blockTargets(o.Cfg) {
		if err := o.refreshBlock(st, t); err != nil {
			return err
		}
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
// interior is still a no-op, and a drifted one still triggers the
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

// PreflightBlocks checks every managed block target for a malformed marker
// pair without writing anything, so `init` can refuse a structurally
// ambiguous file (exit 2) before it seeds or renders. A clean, absent, or
// simply drifted block is fine here — only a broken marker pair is an error.
func PreflightBlocks(repoRoot string, cfg *config.Config) error {
	for _, t := range blockTargets(cfg) {
		content, _, err := readFileIfExists(filepath.Join(repoRoot, filepath.FromSlash(t.rel)))
		if err != nil {
			return err
		}
		if _, _, _, _, lerr := LocateBlock(content); lerr != nil {
			var me *MarkerError
			if errors.As(lerr, &me) {
				me.Path = t.rel
			}
			return lerr
		}
	}
	return nil
}

type blockTarget struct {
	rel      string // repo-root-relative, slash form
	interior string
}

func blockTargets(cfg *config.Config) []blockTarget {
	var ts []blockTarget
	for _, t := range cfg.AgentInstructions.Targets {
		switch t {
		case config.TargetClaude:
			ts = append(ts, blockTarget{rel: "CLAUDE.md", interior: claudeInterior})
		case config.TargetAgents:
			ts = append(ts, blockTarget{rel: "AGENTS.md", interior: agentsInterior})
		}
	}
	return ts
}

func (o Options) refreshBlock(st *State, t blockTarget) error {
	fpath := filepath.Join(o.Root, filepath.FromSlash(t.rel))
	content, _, err := readFileIfExists(fpath)
	if err != nil {
		return err
	}

	found, _, _, curInterior, lerr := LocateBlock(content)
	if lerr != nil {
		var me *MarkerError
		if errors.As(lerr, &me) {
			me.Path = t.rel
		}
		if o.Mode == ModeWarn {
			warnf(o.Stderr, "regen: %s; leaving it untouched", lerr.Error())
			return nil
		}
		return lerr
	}

	desiredHash := hashContent([]byte(t.interior))
	write := func() error {
		out, err := ApplyBlock(content, t.interior)
		if err != nil {
			return err
		}
		if err := writeAtomic(fpath, out); err != nil {
			return err
		}
		st.set(t.rel, desiredHash)
		o.progressf("wrote managed block in %s", t.rel)
		return nil
	}

	switch {
	case found && curInterior == t.interior:
		// Already exactly what we want: record the hash, write nothing.
		st.set(t.rel, desiredHash)
		return nil
	case !found:
		// No block yet — create or append. Not drift.
		return write()
	}

	stored, hasStored := st.get(t.rel)
	if hasStored && stored == hashContent([]byte(curInterior)) {
		// Our own previous interior; the CLI's desired content changed
		// (a version bump) — safe to update without a drift prompt.
		return write()
	}
	return o.handleDrift(st, t.rel, write, stored, hasStored)
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
		ok, err := o.Confirm(fmt.Sprintf("%s drifted from what `constitution init` last wrote; overwrite the managed content?", rel))
		if err != nil {
			return err
		}
		if ok {
			return write()
		}
	}
	return fmt.Errorf(
		"%s drifted from what `constitution init` last wrote; refusing to overwrite it. Re-run with --force to overwrite the managed content (any edits outside the managed region are preserved), or reconcile the file by hand",
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
