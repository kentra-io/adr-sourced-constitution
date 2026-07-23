---
name: constitution-gov
description: Governs how the constitution is used — append-only log, consult-before-planning, amendments only via an ADR under the consent policy. Force-inlines the active constitution into context so governance does not depend on pointer compliance. Applies whenever planning or proposing changes in a governed repo.
---

# constitution-gov

The constitution is the source of truth for this project's standing technical
decisions — the **how** of recurring problems (architecture, mapping, testing,
process) — so that requirements can stay functional and need not re-explain
implementation choices. It is built from a **constitution**: an append-only log
of architectural decision records (ADRs) under `constitution/adr/`, projected
into `constitution/constitution.md`. The projection is **curated** — it renders
only the ADRs that carry a standing rule, and each rule lives in a per-category
`### <category>` subsection as a `#### <slug>` entry, permanently addressable
as `ADR-NNNN/<category>/<slug>`. Point-in-time records stay in the log as
history but are deliberately absent from the constitution, so the ruling
document reads as a concise rulebook, not a decision archive. That projection
is the ruling authority for how work here is planned and built. Load it into
context now:

```
cat constitution/constitution.md
```

If the `constitution` binary is not on PATH (some steps below shell out to it),
install the prebuilt release —
`curl -sSfL https://raw.githubusercontent.com/kentra-io/adr-sourced-constitution/main/install.sh | sh`
— do **not** build from source or install a Go toolchain.

Do this at the start of every planning or architectural task, even if a pointer
already imported it — reading it directly is how you stay governed regardless of
which agent framework you are running under.

## Priority hierarchy (highest wins)

1. **The constitution** (`constitution/constitution.md`). Every active rule in
   it is binding and cites an `ADR-NNNN`.
2. **The human's current instruction**, when it conflicts with a rule — but
   surface the conflict first (see "Amending" below); do not silently break a
   rule to satisfy a request.
3. **Inferred conventions** (what the surrounding code seems to do). These lose
   to the constitution every time. If the codebase drifts from a rule, the rule
   is right and the code is the bug.

## Non-negotiable rules

- **The ADR log is append-only, in both phases.** Never hand-edit a file under
  `constitution/adr/`, and never edit `constitution/constitution.md` — it is a
  generated projection. The only permitted changes to an accepted ADR are its
  `status:` / `superseded-by:` line and, while `phase: draft`, a per-facet
  revision, and only the `constitution` CLI may make any of it. Editing these
  files directly corrupts the log; a `constitution guard` run will flag it.
- **Consult before deciding.** Before proposing a plan, an architecture, or a
  design change, check the constitution for a rule that already governs it. Cite
  the `ADR-NNNN` you are relying on when you do.
- **Amend only through the flow.** If a rule is wrong or in the way, you do not
  override it and you do not edit it by hand. You propose a change through the
  CLI (the `adr-draft` skill) — a whole-ADR supersession, or a targeted
  per-rule retirement via `supersedes-rules`/`removes-rules` that leaves the
  retired ADR's file untouched — and the human approves that write under the
  project's consent policy. Amendment is a governed act, never a hand-edit.

## Retirement and resurrection

A later ADR can retire an individual prior rule — not just supersede a whole
ADR — by citing it in `supersedes-rules` (replaced by a rule of its own) or
`removes-rules` (retired outright). The retired ADR's file never changes;
retirement is fold-time masking applied when `constitution.md` is rendered.
Retirement directives are honored only from the *currently-accepted* ADR that
wrote them: if that ADR is itself later superseded, any rules it had retired
come back into force unless the new, superseding ADR re-retires them — `regen`
warns when this resurrection happens, so treat that warning as something to
check, not ignore.

## Phase note

`constitution.yml` carries `phase: draft | sealed`. While `phase: draft`, the
log is still a mutable working set — `adr edit`/`adr rm` are legitimate CLI
paths for fixing a wrong entry, still gated by the same human consent as every
other write, and the guard only checks parse validity, id uniqueness, and the
category vocabulary. Once `constitution seal` runs, the log is append-only
forever: edit/rm are refused, supersede/deprecate are the only paths, and the
full guard semantics apply. The non-negotiable rules above hold in **both**
phases without exception — draft is a looser change surface, not a suspension
of governance.

Under the strict consent policy in a non-interactive shell, the interactive
`[y/N]` prompt always fails closed; `--approve` is the sanctioned path there,
with the harness's own permission prompt serving as the human gate.

## Why it works this way

Rules live in individual, immutable decision records so their history and
rationale survive; `constitution.md` is only a view of the currently-active,
rule-bearing set. Not every decision is a rule — a decision that establishes no
standing constraint is recorded in the log without a `## Rules` section and
never projects, keeping the constitution a tight list of what actually governs.
That is why edits go through the CLI (it preserves the append-only log and
re-renders the view) and why amendments are new records, not rewrites: the
question "why is this a rule, and what did it replace?" must always be
answerable from the log.
