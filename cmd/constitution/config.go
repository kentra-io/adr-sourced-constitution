package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/config"
)

// configCommand is the `constitution config` subcommand group: machine-
// readable introspection of the constitution.yml vocabulary (issue #18),
// so a skill can read the vocabulary instead of hardcoding it — the
// drift that caused issue #17 ("github" vs the real "github-issue") — plus
// `config set`, so no skill ever hand-serializes YAML to change one field
// (issue #18).
func configCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "introspect and edit the constitution.yml config",
		Commands: []*cli.Command{
			configSchemaCommand(),
			configSetCommand(),
		},
	}
}

// configSchemaCommand implements `constitution config schema`: print
// config.Schema() as indented JSON to stdout. Static — it describes the
// vocabulary this build understands, not any one project's live config,
// so it needs no constitution.yml and no project root.
func configSchemaCommand() *cli.Command {
	return &cli.Command{
		Name:  "schema",
		Usage: "print the constitution.yml config vocabulary as JSON",
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runConfigSchema(cmd)
		},
	}
}

func runConfigSchema(cmd *cli.Command) error {
	enc := json.NewEncoder(cmd.Root().Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(config.Schema())
}

// configSetOwners names, for every constitution.yml key `config set`
// refuses to touch, the verb that owns it instead — so the refusal
// redirects rather than dead-ends. schemaVersion maps to the empty string
// because it has no owning writer at all in this build: issue #18 scopes
// schemaVersion discipline + the v0.1 `phase` migration OUT, and v1 has no
// migration machinery, so there is nothing to redirect to.
var configSetOwners = map[string]string{
	"phase":         "constitution seal",
	"categories":    "adr new --new-category",
	"schemaVersion": "",
}

// configSetCommand implements `constitution config set <key> <value>`
// (issue #18): load the config, apply one dotted key from the settable
// vocabulary, re-validate the WHOLE resulting Config in memory, and only
// then write it atomically — so no skill ever hand-serializes
// constitution.yml.
func configSetCommand() *cli.Command {
	return &cli.Command{
		Name:      "set",
		Usage:     "set one constitution.yml key (validated, atomic write)",
		ArgsUsage: "<key> <value>",
		Description: "Settable keys: consent.policy, sourceTracking.type,\n" +
			"sourceTracking.pattern, agentInstructions.targets, skills.trees (the\n" +
			"last two take a comma-separated list in <value> — config set has one\n" +
			"positional value, not a repeatable flag). <value> must be non-empty:\n" +
			"config set is for explicit values, not silently resetting a key to\n" +
			"its default.\n\n" +
			"Governed keys refuse and name their owning verb instead: phase belongs\n" +
			"to `constitution seal`, categories grow via `adr new --new-category`,\n" +
			"and schemaVersion has no writer at all in this build (no migration\n" +
			"machinery).\n\n" +
			"The whole resulting config is re-validated before anything is written:\n" +
			"an illegal value leaves constitution.yml byte-identical to before —\n" +
			"the failure is at the point of the write, not on some later command.\n\n" +
			"Not consent-gated: config is not the ADR log (matching `init`).",
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runConfigSet(cmd)
		},
	}
}

func runConfigSet(cmd *cli.Command) error {
	if cmd.Args().Len() != 2 {
		return &exitError{
			err:  fmt.Errorf("config set: usage: constitution config set <key> <value>"),
			code: 2,
		}
	}
	key := cmd.Args().Get(0)
	value := cmd.Args().Get(1)
	if value == "" {
		return &exitError{err: fmt.Errorf("config set: %w", errEmptyValue), code: 2}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	configPath := filepath.Join(cwd, "constitution.yml")

	cfg, err := config.LoadLenient(configPath)
	if err != nil {
		return err
	}

	if owner, governed := configSetOwners[key]; governed {
		if owner == "" {
			return &exitError{
				err:  fmt.Errorf("config set: %q is not settable — schemaVersion has no writer in this build (no migration machinery)", key),
				code: 2,
			}
		}
		return &exitError{
			err:  fmt.Errorf("config set: %q is not settable via config set; use %s instead", key, owner),
			code: 2,
		}
	}

	if err := applyConfigKey(cfg, key, value); err != nil {
		return &exitError{err: fmt.Errorf("config set: %w", err), code: 2}
	}

	// Re-validate the WHOLE resulting config in memory before a single byte
	// reaches the real constitution.yml: a refusal here must leave it
	// byte-identical to before — the failure is at the point of the write,
	// not on some later command.
	if err := cfg.Validate(); err != nil {
		return &exitError{err: fmt.Errorf("config set: %w", err), code: 2}
	}

	if err := persistConfig(configPath, cfg); err != nil {
		return err
	}

	// Echo what was actually STORED, not the raw <value>: for the list keys
	// they can differ (comma/whitespace normalized away), and the
	// confirmation must reflect the persisted config, not the input.
	_, err = fmt.Fprintf(cmd.Root().Writer, "set %s = %s\n", key, storedConfigValue(cfg, key))
	return err
}

// errEmptyValue is config set's "explicit values only" refusal: a <value>
// that carries no content is refused rather than silently resetting the
// key to its zero-value/default. Shared verbatim by the literal-empty-string
// fast path in runConfigSet and applyConfigKey's list-key check below, so a
// degenerate list value (",", " , ") — which is not the empty string, but
// still yields zero content after splitConfigList — hits the exact same
// refusal instead of a silent reset.
var errEmptyValue = errors.New(
	"<value> must not be empty — config set is for explicit values, not silently resetting a key to its default")

// applyConfigKey mutates cfg's one settable field named by key. A key not
// in this switch is refused here as unknown — configSetOwners has already
// handled the governed keys (phase, categories, schemaVersion) before this
// is reached, so anything left over is either one of the five settable
// keys or a typo.
func applyConfigKey(cfg *config.Config, key, value string) error {
	switch key {
	case "consent.policy":
		cfg.Consent.Policy = value
	case "sourceTracking.type":
		cfg.SourceTracking.Type = value
	case "sourceTracking.pattern":
		cfg.SourceTracking.Pattern = value
	case "agentInstructions.targets":
		list := splitConfigList(value)
		if len(list) == 0 {
			return errEmptyValue
		}
		cfg.AgentInstructions.Targets = list
	case "skills.trees":
		list := splitConfigList(value)
		if len(list) == 0 {
			return errEmptyValue
		}
		cfg.Skills.Trees = list
	default:
		return fmt.Errorf(
			"unknown key %q (settable: consent.policy, sourceTracking.type, sourceTracking.pattern, agentInstructions.targets, skills.trees)",
			key)
	}
	return nil
}

// storedConfigValue reads back cfg's current value for key, in the same
// dotted-key vocabulary applyConfigKey writes — the display form for
// config set's confirmation line, so it reports what landed in cfg (and,
// after persistConfig, on disk), not the raw <value> argument.
func storedConfigValue(cfg *config.Config, key string) string {
	switch key {
	case "consent.policy":
		return cfg.Consent.Policy
	case "sourceTracking.type":
		return cfg.SourceTracking.Type
	case "sourceTracking.pattern":
		return cfg.SourceTracking.Pattern
	case "agentInstructions.targets":
		return strings.Join(cfg.AgentInstructions.Targets, ",")
	case "skills.trees":
		return strings.Join(cfg.Skills.Trees, ",")
	default:
		return ""
	}
}

// splitConfigList turns config set's single positional <value> into
// constitution.yml's []string shape for the two list-typed settable keys:
// comma-separated, trimmed, empty entries dropped.
func splitConfigList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
