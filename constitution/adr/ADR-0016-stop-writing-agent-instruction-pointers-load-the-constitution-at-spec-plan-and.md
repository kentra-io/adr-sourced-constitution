---
id: ADR-0016
title: Stop writing agent-instruction pointers; load the constitution at spec, plan and ADR time
date: 2026-10-16
status: accepted
---

## Context and Problem Statement
Since v0.1, `init` and `regen` wrote a managed block into CLAUDE.md (an
`@constitution/constitution.md` import) and AGENTS.md (a textual pointer), so
the constitution was loaded into every agent session (the strategy of the
now-deprecated ADR-0001 and ADR-0002). In practice the constitution is a review
utility: it matters when a spec, a plan or an ADR is written and reviewed, and
is noise for implementation, review and general questions.

## Considered Options
- Remove the pointer entirely; the bundled skills load the constitution when a
  spec, a plan or an ADR is written.
- Keep the pointer, opt-in (no targets by default).
- Replace the `@import` with a one-line textual hint in CLAUDE.md.

## Decision Outcome
Chosen: remove the pointer entirely (CLI 0.4.0). `init` and `regen` no longer
write CLAUDE.md or AGENTS.md. Existing blocks are removed on the next `regen`
or mutating verb (a hand-edited block is only warned about), `agentInstructions`
is dropped from `constitution.yml`, and `--target` is accepted but ignored. The
constitution-gov and plan-gate skills read `constitution/constitution.md` when a
spec, a plan or an ADR is written or revised. The opt-in option was rejected
because it leaves existing repos with the block; the textual hint still adds a
line to every session.
