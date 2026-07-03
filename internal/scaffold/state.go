package scaffold

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	yaml "go.yaml.in/yaml/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
)

// StateSchemaVersion pins the constitution/.state format. An unrecognized
// version is refused (no migration machinery in v1), mirroring config's
// stance.
const StateSchemaVersion = 1

// stateFileName is .state's fixed location under constitution/. It is
// CLI-owned bookkeeping, never hand-edited: it records the sha256 of the
// last content the CLI wrote to each managed target (a managed block's
// interior, or a fanned-out skill file), so interior/file drift from what
// the CLI last wrote is detectable. It is not the ADR log and carries no
// tamper-evidence weight — the design's soft check (plan §2.2, §6): the
// user's own files are not the source of truth.
//
// Format (versioned YAML, keys sorted by path for byte-stable, diffable
// output):
//
//	schemaVersion: 1
//	managed:
//	  - path: CLAUDE.md
//	    hash: <sha256-hex of the last-written block interior>
//	  - path: .claude/skills/adr-draft/SKILL.md
//	    hash: <sha256-hex of the last-written file content>
//
// Paths are repo-root-relative and slash-form on every OS, so the file is
// identical across platforms.
const stateFileName = ".state"

// State is the parsed constitution/.state. Managed maps a repo-root-relative
// slash path to the hex sha256 of the last content the CLI wrote there.
type State struct {
	schemaVersion int
	managed       map[string]string
}

// stateFile is the on-disk YAML shape (a sorted slice, not a map, so
// marshaling is deterministic without relying on the library's map-key
// ordering).
type stateFile struct {
	SchemaVersion int          `yaml:"schemaVersion"`
	Managed       []stateEntry `yaml:"managed"`
}

type stateEntry struct {
	Path string `yaml:"path"`
	Hash string `yaml:"hash"`
}

// newState returns an empty State at the current schema version.
func newState() *State {
	return &State{schemaVersion: StateSchemaVersion, managed: map[string]string{}}
}

// statePath returns constitution/.state under root.
func statePath(root string) string {
	return filepath.Join(root, "constitution", stateFileName)
}

// LoadState reads constitution/.state under root. A missing file yields a
// fresh empty State (a repo that has never been init'd has no state yet).
// Corrupt YAML or an unrecognized schemaVersion is returned as an error whose
// message names the .state path and the parse problem; the Refresh engine
// degrades that error to an empty state (loadStateOrEmpty) so a poisoned
// bookkeeping file can never block a refresh, while callers that want to
// surface the problem directly still can.
func LoadState(root string) (*State, error) {
	data, err := os.ReadFile(statePath(root))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return newState(), nil
		}
		return nil, err
	}
	var sf stateFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("%s: not valid YAML: %w", statePath(root), err)
	}
	if sf.SchemaVersion != StateSchemaVersion {
		return nil, fmt.Errorf(
			"%s: unsupported schemaVersion %d (this build only supports %d)",
			statePath(root), sf.SchemaVersion, StateSchemaVersion,
		)
	}
	st := newState()
	for _, e := range sf.Managed {
		st.managed[e.Path] = e.Hash
	}
	return st, nil
}

// get returns the recorded hash for a repo-root-relative slash path.
func (s *State) get(path string) (hash string, ok bool) {
	h, ok := s.managed[path]
	return h, ok
}

// set records the hash for a path.
func (s *State) set(path, hash string) {
	s.managed[path] = hash
}

// empty reports whether the state records no managed targets.
func (s *State) empty() bool { return len(s.managed) == 0 }

// Save atomically (re)writes constitution/.state under root, entries sorted
// by path for byte-stable output.
func (s *State) Save(root string) error {
	sf := stateFile{SchemaVersion: s.schemaVersion}
	for p, h := range s.managed {
		sf.Managed = append(sf.Managed, stateEntry{Path: p, Hash: h})
	}
	sort.Slice(sf.Managed, func(i, j int) bool { return sf.Managed[i].Path < sf.Managed[j].Path })

	data, err := yaml.Marshal(sf)
	if err != nil {
		return err
	}
	return atomicwrite.WriteFile(statePath(root), data, 0o644)
}

// hashContent returns the hex sha256 of b, the canonical drift fingerprint.
func hashContent(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
