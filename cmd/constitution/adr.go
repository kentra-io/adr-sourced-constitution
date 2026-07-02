package main

import "github.com/urfave/cli/v3"

// adrCommand is the `constitution adr` subcommand group (plan §4): the
// verbs that operate on an individual ADR record. `new` and `renumber` live
// here; `supersede` and `deprecate` are top-level because they read as
// verbs on the log, not on the `adr` noun.
func adrCommand() *cli.Command {
	return &cli.Command{
		Name:  "adr",
		Usage: "operate on individual ADR records",
		Commands: []*cli.Command{
			newCommand(),
			renumberCommand(),
		},
	}
}

// approveFlag is the shared consent-bypass flag (plan §2.4): under the
// strict policy every mutating verb accepts --approve for scripted use.
func approveFlag() cli.Flag {
	return &cli.BoolFlag{
		Name:  "approve",
		Usage: "approve the write non-interactively (required under the strict consent policy when stdin is not a terminal)",
	}
}
