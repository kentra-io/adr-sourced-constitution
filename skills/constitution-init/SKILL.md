---
name: constitution-init
description: Interactive, staged founding interview that bootstraps a project's constitution — elicits logistics, business purpose, and settled architecture; triages each answer into a standing rule, a point-in-time record, an unvalidated bet, or nothing at all; then writes one founding ADR via a single `constitution init --founding-file` call. Ends in draft phase — sealing is a later, separate, human-initiated act. Invoke explicitly with /constitution-init.
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
re-run only refreshes integration; it will not re-seed the founding ADR).

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

## The founding roadmap: Found, Validate, Settle — say this before question one

Tell the human, in your own words, before you ask question one: this
interview is only the first of three moments in how this constitution earns
trust, and none of the words below is the `phase:` field in
`constitution.yml` (that field only ever says `draft` or `sealed`) — they are
the human story around it.

- **Found.** This interview, right now. You elicit logistics, business
  purpose, and just enough already-settled architecture to get started; you
  triage every answer into a rule, a point-in-time record, an unvalidated
  bet, or nothing at all; and you write it as one founding ADR. `init` always
  leaves the result in **draft** — cheap to revise (`adr edit`/`adr rm`) —
  deliberately, because nothing flagged as a bet has been checked yet.
- **Validate.** Later, out of band, not part of this flow. Whoever owns each
  flagged bet does the research, spike, or prototype that could still kill
  it.
- **Settle.** Later still, once Validate has run its course: a human runs
  `constitution seal` — the explicit act that ends draft and makes the log
  append-only forever.

Settled answers you elicit during Found become rules right away, in the
founding ADR you write today. Bets stay recorded — never welded into a rule —
until Validate clears them, and get promoted into rule-bearing ADRs during
Settle. Say this plainly now; you'll restate it at the close of the
interview too.

## How this projects into `constitution.md` — explain this before offering categories

Tell the human, in your own words, before you ask about the category list:

- `constitution.md` is organized into **sections**, one per **category**. The
  category vocabulary the human picks *is* the table of contents of the
  rendered constitution — it is not incidental.
- `init` seeds exactly **one** founding ADR (`ADR-0001`, titled "Founding
  constitution") from the founding file you compose together. That one ADR
  can still carry rules in **more than one category** — its `## Rules`
  section groups them by `### <category>` / `#### <slug>` (categories must
  be in the configured vocabulary), and each shows up in its own section of
  the render.
- Every individual rule is permanently addressable as
  `ADR-NNNN/<category>/<slug>` — useful later when superseding or retiring
  just that rule, not the whole ADR.
- Everything in the founding ADR that is **not** the `## Rules` section —
  the mandatory narrative (Context and Problem Statement, Considered
  Options, Decision Outcome) and any other prose section you add, e.g. a
  section recording deferred bets — stays in the log forever (full history,
  auditable) but **never renders** in `constitution.md`. That is by design,
  not a bug — say so explicitly so the human isn't surprised when something
  they told you about "disappears" from the render. It didn't disappear; it
  just isn't a rule.

## Interview — three announced parts, one question at a time

Announce the shape before you start, then work through it one question at a
time, in your own words, confirming each answer: "I'll ask about this in
three parts — first the logistics of how you track work, then what you're
building and why, then just the technical architecture you've already
settled on." **This is high-level context gathering, not requirements
elicitation** — you are not trying to spec the product, only to seed a
founding constitution.

### Part 1 — logistics

1. **Issue tracking.** How do you track issues or decisions today (GitHub
   issues, Jira, none)? This sets `sourceTracking` — pass it straight to
   `init` as `--source-tracking <type>` (and `--source-pattern <regex>` if
   they want to override the default shape), no post-init edit needed. It
   can also become a `process` rule ("every ADR cites an issue").

   Never invent a `--source-tracking` value from the human's wording
   (`github` is not legal; `github-issue` is). Run `constitution config
   schema` and read the `sourceTracking.type` entry's `values` array for the
   current, authoritative list — don't hand-copy a table here, the CLI is
   the source of truth.

   Separately — this is CLI *write-path* behavior, not something `config
   schema` reports, so confirm it against `constitution init --help` rather
   than trusting this note if it ever drifts — leaving `--source-pattern`/
   `sourceTracking.pattern` empty applies a sensible default shape for
   `github-issue` (`#\d+`) and `jira` (`[A-Z]+-\d+`); `generic` enforces
   nothing beyond non-emptiness when no pattern is set.

   **State this consequence before they choose, because it is not reversible
   by a process rule:** the source contract is enforced by the CLI, not by
   convention. Any type other than `none` makes `--source` **mandatory** on
   every future `adr new`/`supersede`, and under `none` passing `--source` is
   **rejected**. There is no "optional citation" setting.
2. **Agent targets.** What agents/tools do you build for — Claude Code,
   other agent frameworks, both? → `--target claude`, `--target agents`, or
   both (the default).
3. **Spec process.** Do you practice spec-driven development? With what
   framework, and how do you manage the spec (e.g. adopting a lifecycle
   tool)? → usually a `process` rule.

### Part 2 — business purpose and product

Ask **several** questions here — this is the part the old interview skipped,
and it's the one most likely to just be background, not a rule:

1. **Purpose.** What is this project for? What problem does it solve, and
   for whom? This is the question the `purpose` category exists for — ask it
   explicitly, don't assume it's implied by the other answers. A settled
   answer becomes a `purpose` rule (or, if it's scene-setting rather than a
   constraint, a point-in-time record).
2. **Users / customers.** Who uses this, or who is it being built for?
3. **Current stage.** Is this greenfield, an MVP, a mature system being
   extended? What stage is the product itself at?
4. Anything else that's already **decided** and shaping priorities right
   now — versus things that are still open questions. This is the bridge to
   triage: what's already decided becomes a rule or a record now; what's
   still a bet gets flagged and stays recorded-only until Validate, then
   gets promoted during Settle.

Most of what comes out of part 2 is context, not ADR material at all — see
the fourth triage outcome below. Say plainly, when you introduce this part,
that settled purpose lands as rules under the `purpose` category — don't
let the human wonder where their answer went.

### Part 3 — technical architecture, bounded to what's already settled

1. **Project structure.** What's the high-level architecture or layout,
   *as already decided* — not as aspiration? → `architecture` rules.
2. **Settled conventions (optional).** Only if something recurring is
   already decided — mapping, testing, error handling, persistence — ask
   for it as a rule about *how*, so requirements can stay functional and
   skip re-explaining implementation. Do not go fishing for conventions that
   haven't been decided yet; skip this question rather than speculate. A
   rule may delegate to a skill, e.g. "hexagonal architecture per
   `kentra-skills:java-hexagonal`" — that's an illustration, not a
   requirement; use whatever the project actually decided.
3. **Category vocabulary.** Only now propose the starter list —
   `purpose, architecture, code-style, testing, process, tooling, security,
   data` — and let the human trim or extend it. Accepting the default means
   no `--category` flags; any deviation means one `--category <name>` per
   chosen category.

   **If the human trims the list, cross-check before you compose anything:**
   walk every rule you triaged as a standing rule so far and confirm its
   category is still in the final vocabulary. Do this check yourself, now,
   before writing the founding file — do not just caveat it and move on.
   This is not optional housekeeping: if a triaged rule uses a category the
   human just dropped, `init` exits 2, and by then `constitution.yml` and
   `constitution/adr/` are already on disk with the trimmed vocabulary
   baked in — every path this skill documents for recovering (re-running
   `init`, hand-editing `constitution.yml`, `config set categories`) is
   closed at that point. Catching the mismatch before you build the command
   is the only cheap fix; resolve it by either dropping the trim or
   reclassifying the rule, and reconfirm with the human before moving on.

## Triage every answer: rule, record, bet, or nothing — never weld a bet into a rule

For each thing that comes out of the interview, make an explicit **four-way**
call with the human. Most of part 2 lands in the last bucket — say so, don't
let everything default into the founding file just because it was said out
loud:

- **Standing rule** — a normative constraint that should govern *future*
  planning ("always X", "never Y", "prefer Z"). Litmus: *"would you want an
  agent planning six months from now to be held to this?"* If yes, distill a
  1–3 line imperative Rule statement and read it back before moving on.
- **Point-in-time record** — a bootstrap decision worth keeping in the log
  (a starting-point choice, an accepted tradeoff, scene-setting context)
  that establishes no ongoing constraint. Goes into the founding ADR's
  narrative; never renders.
- **Unvalidated bet** — a technology or design assumption research could
  still kill. Second litmus: *"could research still kill this?"* If yes,
  this is a bet, **never** a clause welded into the founding ADR's `## Rules`
  section — record it in its own clearly-labeled prose section (never
  rendered) and revisit once Validate has checked it. This has bitten real
  projects before: founding constitutions welded unvalidated tech bets
  straight into rule-bearing ADRs, and later forced a whole-log reset.
- **Nothing — it was background.** Not every true, interesting thing said
  in the interview belongs in the log at all. If it doesn't inform a future
  decision and isn't itself a decision, don't write it anywhere. Most of
  part 2's material ends up here; that's expected, not a failure to elicit
  enough.

Get an explicit call for each candidate before recording it. Writing nothing
at all is allowed (more can be added later with `adr-draft`).

## Write the founding file — only after every answer is confirmed

Grammar (from `constitution init --help`, authoritative): the founding file
is a single MADR body — exactly what `adr new --body-file` takes — and
`init` seeds exactly **one** founding ADR from it, always `ADR-0001`, always
titled "Founding constitution". It must carry all three mandatory sections:

- `## Context and Problem Statement`
- `## Considered Options`
- `## Decision Outcome`

Use these to tell the founding story — purpose, stage, the logistics
answers, whatever was triaged as a point-in-time record. None of this
renders into `constitution.md`; it lives in the log as the ADR's narrative.

It MAY also carry `## Rules`, in the same `### <category>` / `#### <slug>`
grammar `adr new --rule` composes (categories must be in the configured
vocabulary) — one entry per standing rule triaged above, grouped under
whichever categories apply; a rule-bearing founding ADR can span several
categories. Omitting `## Rules` entirely seeds a catalog-only (record-only)
founding ADR.

Bets get their own clearly-named, non-mandatory section — e.g. `## Deferred
bets` — free-form prose. Any section beyond the three mandatory ones and
`## Rules` is preserved **verbatim in the log** but **never rendered**,
which is exactly the mechanism a bet needs: durable, visible to whoever does
the Validate work later, and structurally incapable of masquerading as a
rule.

Write the confirmed content to a temp path **outside the repo**, e.g.
`/tmp/founding.md`:

```
## Context and Problem Statement
This project needs a founding constitution before planning starts, so that
recurring decisions (how we track issues, why the project exists, what's
already architecturally settled) don't get re-litigated or re-explained.

## Considered Options
Capture only settled logistics and purpose now; defer unvalidated technical
choices until they've been checked.

## Decision Outcome
Bootstrap with the logistics, purpose, and settled architecture below.
Postgres was chosen at bootstrap for its maturity and the team's
familiarity — recorded as a bet below, not a rule, until it's validated.

## Rules

### purpose
#### solve-x-for-y
This project exists to solve X for Y.

### architecture
#### error-envelope
All API errors return a typed envelope: `{code, message, details}`.

### testing
#### error-path-coverage
Every handler needs at least one test exercising its error envelope.

## Deferred bets
Postgres as the initial datastore — an unvalidated bet, not a rule. Revisit
once Validate has checked it; do not promote it into a rule until then.
```

Show the human this exact file content and get their explicit confirmation
before writing it to disk, and before assembling the `constitution init`
command.

## Show the command, then run it once

```
constitution init \
  --target claude --target agents \
  --consent strict \
  --source-tracking github-issue \
  --founding-file /tmp/founding.md
```

Include `--source-tracking` whenever interview part 1 question 1 settled on
anything other than "none" — there is no longer a post-init edit needed for
this. Only add `--source-pattern <regex>` if the human wants to *override*
the type's default shape (e.g. `--source-tracking jira --source-pattern
'PROJ-\d+'` to pin one project prefix instead of any `[A-Z]+`) — an empty
`--source-pattern ''` is a no-op identical to omitting the flag, so never
show it in the command unless it carries a real overriding value. Add
`--category <name>` once per category only if the vocabulary deviated from
the starter list. Show the human this exact command before running it. Run
it **exactly once**. `init` writes
`constitution.yml` (`phase: draft`), seeds the one founding ADR, renders
`constitution/constitution.md`, writes the managed pointer blocks, and fans
the Layer-2 skills out to `.claude/`, `.agents/`, `.cursor/`. Do not run it a
second time to "fix" something — if a flag was wrong, tell the human what
happened and adjust deliberately.

## Consent mechanics

`init` itself is not consent-gated, but every *later* mutating verb
(`adr new`, `adr edit`, `adr rm`, `supersede`, `deprecate`, `seal`) is, under
whatever `--consent` policy you just set. Under `strict`, each prompts
`[y/N]` on a real terminal — and in a non-TTY agent shell that prompt **always
fails**. The sanctioned path is `--approve`, with the agent harness's own
command-permission prompt as the actual human consent gate. Never add a
mutating command to an `allowed-tools` list, and never wrap/`eval` the prompt.
`constitution config set` is the exception: it is not consent-gated (config
is not the ADR log), so no `--approve` flag exists for it.

## Verify and hand off

1. If a later config change is needed — the human changes their mind about
   `sourceTracking`, `consent.policy`, `agentInstructions.targets`, or
   `skills.trees` after init — use `constitution config set <key> <value>`,
   never a hand-edit of `constitution.yml`. It validates the whole resulting
   config before writing and refuses cleanly on an illegal value, so there
   is no "invalid until the next command" failure mode to warn about
   anymore. `phase` and `categories` are refused by `config set` with a
   redirect (`constitution seal`, `adr new --new-category`) — that's by
   design, not a bug to work around.
2. Run `constitution guard` — it should report clean.
3. `cat constitution/constitution.md` and show the human the render:
   rule-bearing entries grouped by category, one section per category.
   Everything triaged as a record, a bet, or background is correctly
   absent — it lives in `constitution/adr/` only (or nowhere, for
   background). If every triaged answer was a record, a bet, or nothing, the
   file shows the `No standing rules yet.` placeholder, which is the right
   outcome, not a bug.
4. Point them at `adr-draft` for new decisions and `constitution-gov` for
   planning governance. Close by restating the roadmap: **Found is done,
   Validate is next** — whoever owns each flagged bet does that work out of
   band — and **Settle is a human explicitly running `constitution seal`**
   once Validate clears. Until then the log stays mutable
   (`adr edit`/`adr rm`), which is enforcement, not a hedge: `phase: draft`
   is what keeps the door open, and `seal` is the only thing that ever
   closes it.

## Never

- Never run `constitution init` more than once in this flow.
- Never hand-edit `constitution.yml`, `constitution/adr/`, or
  `constitution/constitution.md`. Config changes go through `constitution
  config set`; log changes go through `adr new`/`adr edit`/`adr rm`/
  `supersede`/`deprecate`; the render is generated.
- Never invent a config value from the human's phrasing. Every field in
  `constitution.yml` is a closed vocabulary validated by the CLI, so a
  plausible-sounding guess (`type: github`) is not a near-miss — it breaks
  immediately, because `config set` and `init` both validate before writing.
  Get the current legal values from `constitution config schema`, not from
  memory or a table in this file.
- Never write the founding file, or run any command, before the human has
  confirmed its exact content.
- Never pre-approve or bypass a mutating command's consent prompt.
- Never weld an unvalidated technology bet into the founding ADR's
  `## Rules` section — keep it in its own non-mandatory, never-rendered
  section until Validate checks it.
- Never treat every part-2 answer as ADR material — most of it is
  background; use the fourth triage outcome and write nothing for it.
- Never run `constitution seal` from this flow — sealing is a separate,
  later, human-initiated act (Settle, not Found).
