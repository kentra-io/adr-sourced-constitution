---
name: constitution-init
description: Interactive, staged founding interview that bootstraps a project's constitution — elicits agent targets, consent policy, source tracking, spec process, architecture, and founding principles before writing anything; triages each principle into a standing rule, a point-in-time record, or an unvalidated bet; then seeds them via one `constitution init --founding-file` call. Ends in draft phase — sealing is a later, separate, human-initiated act. Invoke explicitly with /constitution-init.
disable-model-invocation: true
---

# constitution-init

Bootstrap this repository's constitution through a real conversation, not a
form. This is a **staged founding interview**: elicit fully, get the human's
explicit confirmation, and only then write. Ask one thing at a time, in your
own words, and confirm each answer before moving on. Run the interview in the
normal (non-forked) context so you can actually talk to the human.

First check you are not clobbering an existing setup: if `constitution.yml`
already exists, stop and tell the human this repo is already initialized (a
re-run only refreshes integration; it will not re-seed founding ADRs).

## Ensure the `constitution` CLI is available

Every step here shells out to `constitution`. If it is not on PATH (a fresh
container often has no binary), install the **prebuilt release** — do **not**
build from source or install a Go toolchain (that is only for developing the
primitive itself):

```
curl -sSfL https://raw.githubusercontent.com/kentra-io/adr-sourced-constitution/main/install.sh | sh
```

This is arch-aware, checksum-verified, and installs to `~/.local/bin` (no root).
Add that dir to PATH if the script warns it is missing. For containers, prefer
baking the binary into the image (see the repo's `docs/releasing.md`).

## The elicitation contract — read this first, it is binding

You **MUST** interview before you write, and you **MUST NOT** compose the
founding file, edit `constitution.yml`, or run `constitution init` (or any
other command) until the human has explicitly confirmed the exact content
you are about to write. The founding file is composed *only* from confirmed
answers — never from your own guesses about what the project probably wants.

This has already failed twice, for real: once an agent formalized before
eliciting — it wrote the founding file from its own guesses instead of
asking — and once a human was blindsided by the rendered `constitution.md`
because nobody explained, before the render, that its sections *are* the
category vocabulary. Fix: interview fully, confirm before writing, and
explain the projection model (next) before you ever ask about categories.

## How this projects into `constitution.md` — explain this before offering categories

Tell the human, in your own words, before you ask about the category list:

- `constitution.md` is organized into **sections**, one per **category**. The
  category vocabulary the human picks *is* the table of contents of the
  rendered constitution — it is not incidental.
- One founding principle can carry rules in **more than one category** (the
  `## Rules` → `### <category>` → `#### <slug>` grammar below): a principle
  about "how we build APIs" might drop one rule under `architecture` and
  another under `testing`, and both show up in their respective sections.
- Every individual rule is permanently addressable as
  `ADR-NNNN/<category>/<slug>` — useful later when superseding or retiring
  just that rule, not the whole ADR.
- A principle with **no** `## Rules` block is record-only: it stays in the
  log forever (full history, auditable) but **never renders** in
  `constitution.md`. That is by design, not a bug — say so explicitly so the
  human isn't surprised when a principle "disappears" from the render.

## Interview — an opinionated question catalog, one at a time

Don't just offer a bare category list — ask real questions and let each one
earn its place in the config or the founding file:

1. **Issue tracking.** How do you track issues or decisions today (GitHub
   issues, Jira, none)? This becomes a post-init edit to `constitution.yml`'s
   `sourceTracking` block, and sometimes a `process` rule ("every ADR cites an
   issue"). **Note the limitation honestly:** `init` always writes
   `sourceTracking.type: none` regardless of the answer — tell them you'll
   set `type:`/`pattern:` *after* init, since that's config, not the log.

   `type:` accepts **exactly four values** — copy one verbatim, never invent a
   value from the human's wording (`github` is not legal; `github-issue` is):

   | They answer | `type:` | default `pattern:` | example `--source` |
   | --- | --- | --- | --- |
   | GitHub issues | `github-issue` | `#\d+` | `#42` |
   | Jira | `jira` | `[A-Z]+-\d+` | `PROJ-42` |
   | Something else (Linear, a wiki, …) | `generic` | none — **any non-empty string passes** | `LIN-42`, `conversation` |
   | Nothing / don't force it | `none` | n/a | must be omitted |

   Setting `pattern:` overrides the default for any type; leave it `""` to keep
   the default. Under `generic` an empty pattern enforces *nothing* beyond
   non-emptiness — if they want real enforcement, give them `github-issue`/`jira`
   or an explicit `pattern:`.

   **State this consequence before they choose, because it is not reversible by
   a process rule:** the source contract is enforced by the CLI, not by
   convention. Any type other than `none` makes `--source` **mandatory** on
   every future `adr new`/`supersede`, and under `none` passing `--source` is
   **rejected**. There is no "optional citation" setting — declining a standing
   rule about citing issues does not make `--source` optional.
2. **Agent targets.** What agents/tools do you build for — Claude Code,
   other agent frameworks, both? → `--target claude`, `--target agents`, or
   both (the default).
3. **Spec process.** Do you practice spec-driven development? With what
   framework, and how do you manage the spec (e.g. adopting a lifecycle
   tool)? → usually a `process` rule.
4. **Project structure.** What's the high-level architecture or layout? →
   `architecture` rules.
5. **Recurring "hows."** How do you solve recurring problems — mapping,
   testing, error handling, persistence? The goal is rules about *how*, so
   requirements can stay functional and skip re-explaining implementation. A
   rule may delegate to a skill, e.g. "hexagonal architecture per
   `kentra-skills:java-hexagonal`" — that's an illustration, not a
   requirement; use whatever the project actually decided.
6. **Category vocabulary.** Only now propose the starter list —
   `purpose, architecture, code-style, testing, process, tooling, security,
   data` — and let the human trim or extend it. Accepting the default means
   no `--category` flags; any deviation means one `--category <name>` per
   chosen category.

## Triage every principle: rule, record, or bet — never weld a bet into a rule

For each founding principle that comes out of the interview, make an explicit
**three-way** call with the human:

- **Standing rule** — a normative constraint that should govern *future*
  planning ("always X", "never Y", "prefer Z"). Litmus: *"would you want an
  agent planning six months from now to be held to this?"* If yes, distill a
  1–3 line imperative Rule statement and read it back before moving on.
- **Point-in-time record** — a bootstrap decision worth keeping in the log
  (a starting-point choice, an accepted tradeoff) that establishes no ongoing
  constraint. Stays in the log; never renders.
- **Unvalidated bet** — a technology or design assumption research could
  still kill. Second litmus: *"could research still kill this?"* If yes, this
  is a bet, **never** a clause welded into a rule-bearing structural ADR —
  record it as a record-only ADR (or hold it on a parked list) and revisit
  once stage 2 has validated it. This has bitten real projects before:
  founding constitutions welded unvalidated tech bets straight into
  structural rule ADRs, and later forced a whole-log reset.

Get an explicit yes for each principle's classification before recording it.
Zero founding principles is allowed (more can be added later with `adr-draft`).

## The staged founding roadmap — this interview is stage 1 only

Set expectations up front and again at the end:

1. **Stage 1 (this interview):** purpose + very-high-level design ADRs only.
2. **Stage 2 (later, out of band):** research whatever got flagged as an
   unvalidated bet above.
3. **Stage 3 (later):** technical-architecture ADRs informed by that
   research, superseding or refining stage-1 records as needed — then
   `constitution seal` once the bets behind them are validated.

Do not sell finality: `init` always ends **in draft**. While draft, `adr edit
<id>` (per-facet revision) and `adr rm <id>` keep changes cheap; both are
consent-gated and both are refused after `constitution seal` makes the log
append-only forever. Say this plainly at the close of the interview, not just
in passing.

## Write the founding file — only after every answer is confirmed

Grammar (from `constitution init --help`, authoritative): one principle per
`## <title>` heading — the heading becomes the ADR title, the text beneath it
becomes its Decision Outcome. A `## Rules` heading immediately following a
principle carries that principle's standing rules, in `### <category>` /
`#### <slug>` form (categories must be in the configured vocabulary); one
`## Rules` block can span several `### <category>` sections. A principle with
no `## Rules` heading seeds a catalog-only (record-only) ADR. There is no
`--principle` flag — the founding file is the only founding input this skill
uses, because it is the only form that can express categories.

Write the confirmed content to a temp path **outside the repo**, e.g.
`/tmp/founding.md`:

```
## Solve API errors the same way everywhere
Consistent error handling keeps client integrations simple.

## Rules

### architecture
#### error-envelope
All API errors return a typed envelope: `{code, message, details}`.

### testing
#### error-path-coverage
Every handler needs at least one test exercising its error envelope.

## Adopted Postgres as the initial datastore
Chosen at bootstrap for its maturity and the team's familiarity — an
unvalidated bet, not a rule; no "## Rules" section, so this stays a
record-only ADR that never appears in constitution.md.
```

Show the human this exact file content and get their explicit confirmation
before writing it to disk, and before assembling the `constitution init`
command.

## Show the command, then run it once

```
constitution init \
  --target claude --target agents \
  --consent strict \
  --founding-file /tmp/founding.md
```

Add `--category <name>` once per category only if the vocabulary deviated
from the starter list. Show the human this exact command before running it.
Run it **exactly once**. `init` writes `constitution.yml` (`phase: draft`),
seeds one founding ADR per principle, renders `constitution/constitution.md`,
writes the managed pointer blocks, and fans the Layer-2 skills out to
`.claude/`, `.agents/`, `.cursor/`. Do not run it a second time to "fix"
something — if a flag was wrong, tell the human what happened and adjust
deliberately.

## Consent mechanics

`init` itself is not consent-gated, but every *later* mutating verb
(`adr new`, `adr edit`, `adr rm`, `supersede`, `deprecate`, `seal`) is, under
whatever `--consent` policy you just set. Under `strict`, each prompts
`[y/N]` on a real terminal — and in a non-TTY agent shell that prompt **always
fails**. The sanctioned path is `--approve`, with the agent harness's own
command-permission prompt as the actual human consent gate. Never add a
mutating command to an `allowed-tools` list, and never wrap/`eval` the prompt.

## Verify and hand off

1. If they asked for issue tracking, edit `constitution.yml`'s
   `sourceTracking` block now — set `type:` to one of the four legal values
   from interview question 1 (`none`/`generic`/`github-issue`/`jira`) and an
   optional `pattern:`. Future ADRs will then require `--source`.
2. Run `constitution guard` — it should report clean. **Run it after the
   config edit, not before**: `constitution.yml` is hand-written here, so this
   is the only check that the value you typed is actually legal. An invalid
   `type:` does not fail at edit time — it fails on the *next* command the
   human runs, long after this flow has handed off. If guard reports
   `field "sourceTracking.type": must be one of …`, you invented a value; fix
   it against the table and re-run.
3. `cat constitution/constitution.md` and show the human the render:
   rule-bearing entries grouped by category, one section per category.
   Record-only principles are correctly absent — they live in
   `constitution/adr/` only. If every principle was record-only or a bet, the
   file shows the `No standing rules yet.` placeholder, which is the right
   outcome, not a bug.
4. Point them at `adr-draft` for new decisions and `constitution-gov` for
   planning governance. Close by restating: **the constitution is still
   draft** — `adr edit`/`adr rm` keep changes cheap — until stage 2's research
   validates the flagged bets and a human explicitly runs `constitution
   seal`.

## Never

- Never run `constitution init` more than once in this flow.
- Never hand-edit `constitution/adr/` or `constitution/constitution.md`.
- Never invent a `constitution.yml` value from the human's phrasing. Every
  field there is a closed vocabulary validated by the CLI, so a plausible-
  sounding guess (`type: github`) is not a near-miss — it breaks every
  subsequent command with a validation error. Copy the exact value from the
  table in interview question 1, then prove it with `constitution guard`.
- Never write the founding file, or run any command, before the human has
  confirmed its exact content.
- Never pre-approve or bypass a mutating command's consent prompt.
- Never weld an unvalidated technology bet into a rule-bearing structural
  ADR — keep it record-only until stage 2 validates it.
- Never run `constitution seal` from this flow — sealing is a separate,
  later, human-initiated act.
