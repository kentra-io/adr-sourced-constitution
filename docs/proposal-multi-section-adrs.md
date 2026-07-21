# Proposal / TODO: allow one ADR to project into multiple constitution sections

**Status:** absorbed 2026-07-19 into
[`proposal-v0.2-next-iteration.md`](./proposal-v0.2-next-iteration.md) —
direction approved (option A, per-rule `###` category subsections); kept for
the option analysis. Raised 2026-07-10 from real friction founding the
`kafka-dq` project's constitution.

## Problem

Today each ADR carries exactly one `--category`, and `constitution.md` groups
ADRs under one `## <category>` heading — so an ADR's entire `## Rule` block lands
in a single section. This forces a hard, sometimes unwanted, coupling between
"how many decisions I record" and "how many sections my constitution has."

Concretely: a user founding a project wanted a **single founding-charter ADR**
whose rules populate several sections (`purpose`, `architecture`, `testing`,
`process`). That is currently impossible — the charter had to be split into one
ADR per category to get a sectioned constitution, or collapsed into one section
to keep one ADR. You cannot have "one decision record, rules fanned across
sections." The split is often the *right* call for supersession granularity, but
the tool should not *force* it; a genuinely single decision (a founding charter,
a cross-cutting policy) can legitimately want presence in several sections.

## Desired behavior

A rule-bearing ADR may declare rules belonging to **more than one category**, and
the projection routes each rule to its category's section in `constitution.md`.
Single-category ADRs keep working exactly as today (backward compatible).

## Design sketch (options to weigh — not decided)

- **A. Per-rule category tags inside `## Rule`.** e.g. sub-headings
  (`### architecture`, `### testing`) or line-prefixes (`[testing] …`) that the
  projector parses and fans out. Keeps `--category` as the ADR's *primary* filing
  but lets individual rules override. Most expressive; needs a grammar decision.
- **B. Repeatable `--category`.** ADR filed under N categories; its whole Rule
  block projects into each. Simple, but duplicates the block across sections —
  usually not what you want.
- **C. Structured rule entries** (one rule = one object with `category` + `text`),
  authored in the body under a machine-readable list. Cleanest projection; biggest
  authoring change.

Leaning A (per-rule tags) — least disruptive to the MADR body, keeps the ADR the
unit of decision while letting the *projection* be section-aware.

## Constraints to respect

- **Immutability / manifest.** The ADR body stays frozen and hash-chained; only
  the *projection* logic changes, so existing logs re-render unchanged. Verify the
  from-empty replay guard still holds with the new projector.
- **Category validation.** Every per-rule category must be in the configured
  vocabulary (`constitution.yml`), same as `--category` today.
- **Supersession is still whole-ADR.** Superseding *one* rule of a multi-section
  ADR still supersedes the entire record — document this as a known limitation, or
  it becomes an argument for keeping decisions granular anyway.
- **Category ordering** in the rendered constitution should follow
  `constitution.yml`'s vocabulary order, not ADR order, so sections are stable.

## Acceptance

- A single ADR with rules tagged `purpose` + `architecture` + `testing` +
  `process` renders four populated sections from one record.
- Existing single-category ADRs render byte-identically to before.
- `regen` + the replay/manifest guard pass on a log containing a multi-section ADR.
