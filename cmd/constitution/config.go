package main

import (
	"context"
	"encoding/json"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/adr-sourced-constitution/internal/config"
)

// configCommand is the `constitution config` subcommand group: machine-
// readable introspection of the constitution.yml vocabulary (issue #18),
// so a skill can read the vocabulary instead of hardcoding it — the
// drift that caused issue #17 ("github" vs the real "github-issue").
func configCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "introspect the constitution.yml config schema",
		Commands: []*cli.Command{
			configSchemaCommand(),
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
