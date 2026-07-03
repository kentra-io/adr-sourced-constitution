package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/urfave/cli/v3"
	yaml "go.yaml.in/yaml/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/adr"
	"github.com/kentra-io/adr-sourced-constitution/internal/atomicwrite"
	"github.com/kentra-io/adr-sourced-constitution/internal/config"
)

// bootstrapSource is the reserved source value founding ADRs use (plan
// §2.8); it satisfies a non-"none" sourceTracking config without matching
// the configured issue/ticket pattern.
const bootstrapSource = "bootstrap"

// mutContext is the shared state a mutating verb needs: the repo root, its
// loaded config, the ADR directory, and the io streams. openRepo builds it
// once at the top of each verb.
type mutContext struct {
	root    string
	adrDir  string
	cfg     *config.Config
	stdout  io.Writer
	stderr  io.Writer
	stdin   io.Reader
	approve bool
}

// openRepo loads the repo rooted at the process working directory: the
// config must exist and be valid, or the verb refuses before touching the
// log.
func openRepo(cmd *cli.Command) (*mutContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(filepath.Join(cwd, "constitution.yml"))
	if err != nil {
		return nil, err
	}
	return &mutContext{
		root:    cwd,
		adrDir:  filepath.Join(cwd, "constitution", "adr"),
		cfg:     cfg,
		stdout:  cmd.Root().Writer,
		stderr:  cmd.Root().ErrWriter,
		stdin:   os.Stdin,
		approve: cmd.Bool("approve"),
	}, nil
}

// gate builds the consent gate for this repo/command, detecting whether
// stdin is an interactive terminal.
func (m *mutContext) gate() consentGate {
	return consentGate{
		policy:  m.cfg.Consent.Policy,
		approve: m.approve,
		isTTY:   isTerminal(os.Stdin),
		in:      m.stdin,
		out:     m.stderr,
	}
}

// regen re-derives constitution.md and the manifest after a mutation.
func (m *mutContext) regen() error {
	return regenAt(m.root, m.stdout, m.stderr)
}

// checkCategory validates a verb's --category against the configured
// vocabulary (plan §2.5): an unknown category is a hard error unless
// --new-category is given. It does NOT persist — it returns isNew so the
// caller can defer the config write until after the consent gate, keeping
// the "refuse ⇒ nothing written" invariant. Passing --new-category for a
// category that already exists is a no-op (isNew=false).
func (m *mutContext) checkCategory(category string, newCategory bool) (isNew bool, err error) {
	for _, c := range m.cfg.Categories {
		if c == category {
			return false, nil
		}
	}
	if !newCategory {
		return false, fmt.Errorf(
			"unknown category %q: not in the configured vocabulary %s; pass --new-category to introduce it",
			category, formatCategories(m.cfg.Categories),
		)
	}
	return true, nil
}

// appendCategory appends category to the vocabulary and atomically rewrites
// constitution.yml. Called only after the consent gate has passed, so a
// refused mutation never touches the config (plan §2.5, still an ordinary
// ADR — no meta-record type).
func (m *mutContext) appendCategory(category string) error {
	m.cfg.Categories = append(m.cfg.Categories, category)
	return persistConfig(filepath.Join(m.root, "constitution.yml"), m.cfg)
}

// readBody reads a MADR body from a file path, or from stdin when path is
// "-" (plan §2.3).
func readBody(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

// applyRuleFlag reconciles the --rule flag with a body-file (plan §2.12,
// shared by `adr new` and `supersede`). --rule composes a "## Rule" section
// as the last body section, making the ADR rule-bearing. A body-file MAY
// instead carry its own "## Rule" section; supplying BOTH is an error (the
// intent is ambiguous). When neither is present the ADR is a catalog-only
// record. The empty/whitespace-only check is deferred to adr.ValidateBody,
// which the callers run next; the plain-prose check (no Markdown heading
// lines) runs here on the raw flag value, before any composition, so a
// heading-bearing rule is refused before a file is written rather than
// silently truncated at projection time (plan §2.12).
func applyRuleFlag(cmd *cli.Command, body []byte) ([]byte, error) {
	if !cmd.IsSet("rule") {
		return body, nil
	}
	rule := cmd.String("rule")
	if err := adr.ValidateRuleText(rule, "--rule"); err != nil {
		return nil, err
	}
	if adr.HasRuleSection(body) {
		return nil, fmt.Errorf(
			"both --rule and a --body-file that already contains a \"## Rule\" section were supplied; provide the rule exactly once (drop --rule, or remove the section from the body)")
	}
	return adr.AppendRuleSection(body, rule), nil
}

// validateSource enforces the source-ref contract (plan §2.8) against the
// project's sourceTracking config: forbidden when type is none, required
// (and pattern-checked) otherwise. The reserved bootstrap source always
// passes when a source is expected.
func validateSource(st config.SourceTracking, source string) error {
	if st.Type == config.SourceTrackingNone {
		if source != "" {
			return fmt.Errorf("--source is not allowed when sourceTracking.type is %q", config.SourceTrackingNone)
		}
		return nil
	}
	if source == "" {
		return fmt.Errorf("--source is required when sourceTracking.type is %q", st.Type)
	}
	if source == bootstrapSource {
		return nil
	}

	pattern := st.Pattern
	if pattern == "" {
		switch st.Type {
		case config.SourceTrackingGitHubIssue:
			pattern = `#\d+`
		case config.SourceTrackingJira:
			pattern = `[A-Z]+-\d+`
		case config.SourceTrackingGeneric:
			return nil // no default shape; any non-empty source is accepted
		}
	}
	re, err := regexp.Compile("^(?:" + pattern + ")$")
	if err != nil {
		return fmt.Errorf("sourceTracking.pattern %q is not a valid regexp: %w", pattern, err)
	}
	if !re.MatchString(source) {
		return fmt.Errorf("--source %q does not match the required pattern %q", source, pattern)
	}
	return nil
}

// persistConfig atomically rewrites constitution.yml from cfg. Used only by
// --new-category, the one path that mutates config. v1 re-serializes the
// struct (comments in a hand-authored config are not preserved) — an
// acceptable tradeoff since the config is CLI-managed.
func persistConfig(path string, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return atomicwrite.WriteFile(path, data, 0o644)
}

// formatCategories renders the vocabulary as "[a, b, c]" for error messages,
// matching the phrasing internal/render uses for the same check on the read
// path.
func formatCategories(categories []string) string {
	out := "["
	for i, c := range categories {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out + "]"
}

// isTerminal reports whether f is an interactive character device. Used to
// pick between prompting and refusing under the strict consent policy.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
