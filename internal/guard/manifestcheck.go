package guard

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/manifest"
)

// checkManifest cross-checks every ADR recorded in
// constitution/adr/.manifest.sha256 against that ADR's CURRENT
// frozen-content hash (internal/manifest.Hash) computed from disk right
// now. This is the only guard check that needs no git at all, and the only
// one that survives a rewritten git history (see the package doc): every
// mutating `constitution` command rewrites the manifest as its last step,
// so an edit made outside the CLI — including one whose containing commit
// was later amended/rebased away, which leaves git mode nothing to diff —
// leaves the manifest holding a stale hash for that file. It is advisory
// (plan §2.7, docs/manifest-canonicalization.md): a committer who edits
// both the ADR and the manifest together in one commit defeats it.
func checkManifest(adrDir string, adrs []adr.ADR) ([]Violation, error) {
	recorded, err := readManifest(adrDir)
	if err != nil {
		if os.IsNotExist(err) {
			if len(adrs) == 0 {
				return nil, nil // nothing adopted yet; nothing to check
			}
			return nil, fmt.Errorf(
				"guard: %s not found (run `constitution regen` to create it)",
				filepath.Join(adrDir, manifest.FileName),
			)
		}
		return nil, err
	}

	byFile := make(map[string]adr.ADR, len(adrs))
	for i := range adrs {
		byFile[filepath.Base(adrs[i].Path)] = adrs[i]
	}

	var violations []Violation
	for _, rec := range recorded {
		a, present := byFile[rec.filename]
		if !present {
			violations = append(violations, Violation{
				Kind: KindFileDeleted, ID: idFromFilename(rec.filename), File: rootRelFile(rec.filename),
				Message: fmt.Sprintf("%s: recorded in %s but no longer present on disk", rec.filename, manifest.FileName),
			})
			continue
		}
		if got := manifest.Hash(a); got != rec.hash {
			violations = append(violations, Violation{
				Kind: KindManifestMismatch, ID: a.ID, File: rootRelFile(rec.filename),
				Message: fmt.Sprintf(
					"%s: on-disk frozen-content hash %s does not match the hash recorded in %s (%s); the file changed without going through the constitution CLI",
					a.ID, got, manifest.FileName, rec.hash,
				),
			})
		}
	}
	return violations, nil
}

// manifestRecord is one parsed line of the sha256sum-style manifest file
// (internal/manifest.Render's format: "<hex>  <filename>").
type manifestRecord struct{ hash, filename string }

func readManifest(adrDir string) ([]manifestRecord, error) {
	data, err := os.ReadFile(filepath.Join(adrDir, manifest.FileName))
	if err != nil {
		return nil, err
	}

	var recs []manifestRecord
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		// internal/manifest.Render always writes "<hex>  <filename>" (two
		// spaces, sha256sum-style); tolerate a lone stray line rather than
		// fail the whole guard run on it.
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			continue
		}
		recs = append(recs, manifestRecord{hash: parts[0], filename: parts[1]})
	}
	return recs, sc.Err()
}

// filenameIDPattern extracts the "ADR-NNNN" id encoded in an ADR filename
// (mirrors internal/adr's unexported filenamePattern; duplicated rather
// than exported since this is the only guard use, and it's a one-line
// regex, not worth widening adr's API surface for).
var filenameIDPattern = regexp.MustCompile(`^(ADR-[0-9]{4,})-`)

// idFromFilename best-effort extracts the id a filename encodes, for a
// violation whose file no longer exists on disk to parse. Falls back to
// the bare filename when it doesn't match the convention at all (should not
// happen for anything internal/manifest ever wrote).
func idFromFilename(name string) string {
	if m := filenameIDPattern.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	return name
}
