package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/urfave/cli/v3"
	yaml "go.yaml.in/yaml/v3"

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

// readBody reads a MADR body from a file path, or from stdin when path is
// "-" (plan §2.3).
func readBody(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
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
// init's config authoring. It re-serializes the struct (comments in a
// hand-authored config are not preserved) — an acceptable tradeoff since
// the config is CLI-managed.
func persistConfig(path string, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return atomicwrite.WriteFile(path, data, 0o644)
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
