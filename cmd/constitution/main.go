// Command constitution maintains an append-only MADR architectural decision
// record (ADR) log and renders it to a deterministic constitution.md
// projection. See adr-sourced-constitution.md in the repo root for the
// design and implementation-plan.md for the build plan.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	cmd := &cli.Command{
		Name:    "constitution",
		Usage:   "maintain an ADR log and its constitution.md projection",
		Version: buildVersion(),
		Commands: []*cli.Command{
			regenCommand(),
		},
	}
	return cmd.Run(ctx, args)
}
