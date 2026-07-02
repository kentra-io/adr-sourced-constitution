package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/kentra-io/adr-sourced-constitution/internal/config"
)

// consentGate decides whether a mutating command may proceed under the
// project's consent policy (plan §2.4). Under "strict" every mutation needs
// an explicit human OK: either --approve (for scripted use) or an
// interactive "yes" at the prompt. Non-interactive with no --approve is a
// hard refusal — nothing is written. Under "off" there is no gate.
//
// The honest limitation (plan §2.4): Layer 1 cannot verify a *human* typed
// the confirmation; the real architectural checkpoint is the agent-harness
// permission prompt around the Bash call. This gate is the CLI-level
// backstop that makes an unattended write refuse by default.
type consentGate struct {
	policy  string    // config.ConsentStrict | config.ConsentOff
	approve bool      // the --approve flag
	isTTY   bool      // stdin is an interactive terminal
	in      io.Reader // where an interactive confirmation is read from
	out     io.Writer // where the prompt is written
}

// confirm returns nil if the mutation may proceed, or a refusal error
// (mapped to a non-zero exit by main) otherwise. verb names the action for
// the prompt and the error message.
func (g consentGate) confirm(verb string) error {
	if g.policy == config.ConsentOff {
		return nil
	}
	if g.approve {
		return nil
	}
	if !g.isTTY {
		return fmt.Errorf(
			"consent: %s requires confirmation under the %q policy, but stdin is not a terminal; pass --approve to proceed non-interactively (or set consent.policy: off)",
			verb, config.ConsentStrict,
		)
	}

	_, _ = fmt.Fprintf(g.out, "About to %s. This writes to the ADR log. Proceed? [y/N] ", verb)
	line, _ := bufio.NewReader(g.in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return fmt.Errorf("consent: %s not confirmed; nothing was written", verb)
	}
}
