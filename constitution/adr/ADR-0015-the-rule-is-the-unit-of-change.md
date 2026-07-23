---
id: ADR-0015
title: The rule is the unit of change
date: 2026-07-23
status: accepted
---

## Context and Problem Statement

v0.1 tied one ADR to exactly one category and, in effect, one implicit rule:
superseding an ADR superseded everything it stood for as a single unit, and a
founding decision that genuinely spanned several concerns had no way to fan
out into the several constitution sections it actually belonged in. One of
the two kafka-dq constitution wipes was exactly this expectation gap — a
multi-concern founding decision that the v0.1 grammar could not represent as
written, forcing a reset instead of a correction.

## Decision Drivers

- Individual rules need their own identity and their own lifecycle,
  independent of the ADR that first introduced them.
- The rendered constitution must stay a pure function of the currently
  accepted ADR set — retiring a rule must not require touching the file that
  originally recorded it.
- Files under `constitution/adr/` are append-only; once an ADR is accepted,
  its body is frozen, so retirement of one of its rules has to be declared
  somewhere else.

## Considered Options

- Keep whole-ADR granularity: simplest, but this is the status quo that
  caused the problem.
- Inline per-rule annotations directly in the body's headings (e.g. marking
  a heading as retired in place): keeps retirement next to the rule, but
  requires editing already-accepted, supposedly-frozen files.
- A uniform per-rule grammar (`## Rules` → `### <category>` → `#### <slug>`)
  with retirement declared in a later, amending ADR's frontmatter
  (`supersedes-rules:` / `removes-rules:`): rules get stable addresses and
  retirement never touches history.

## Decision Outcome

Chosen option: the uniform h3/h4 Rules grammar with frontmatter-declared
retirement. `## Rules` holds one or more `### <category>` subsections, each
holding one or more `#### <slug>` entries; the per-ADR `category:`
frontmatter field is removed, since a rule's category now lives with the
rule itself. Every rule is addressable as `ADR-NNNN/<category>/<slug>`.
A later, amending ADR retires specific prior rules via `supersedes-rules:`
(this ADR's own rules replace them) or `removes-rules:` (the constraint
simply stops applying) — the retired ADR's file is never touched; retirement
is fold-time masking. Retirement counts only from currently-accepted ADRs,
so superseding an amending ADR resurrects whatever it had retired (the
projection stays a pure function of the active set; `regen` warns when this
happens). Whole-ADR supersede/deprecate remains available for decisions that
are fully dead, not just partially amended. There is no backward
compatibility with the v0.1 grammar: the only adopters were in-house and
were hand-migrated.

## Consequences

A single ADR can now project into every relevant constitution section at
once — architecture, testing, process, and more — from one record. Later
ADRs can amend one rule of an earlier, multi-rule ADR without touching or
re-litigating the rest of it. This is a deliberate breaking change to the
ADR schema and grammar; every adopting log had to be hand-migrated with no
tooling provided.
