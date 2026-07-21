# Proposal: v0.2 — founding phase, multi-category rules, purposeful projection

**Status:** direction approved 2026-07-19 (user decisions recorded below);
implementation not started. Supersedes-and-absorbs
[`proposal-multi-section-adrs.md`](./proposal-multi-section-adrs.md).
Provenance: harness `tasks/constitution-init-retro.md` (two kafka-dq
constitution wipes, 2026-07-10 → 07-13, + continued dogfood notes 07-14 and
07-19) and `tasks/lifecycle-refine-retro.md` (the systemic
formalize-before-elicit pattern).

## The goal, stated once

The constitution is the **source of truth for a project's universal technical
decisions — the "how"**: how do we handle mapping, how do we structure tests,
how do we track issues, how do we manage specs. Its job is to support the
planning process so that whoever writes requirements can stay **functional**
and need not re-explain implementation choices in every issue. Minimizing that
leakage is the design objective every change below serves.

## Decisions

Locked by the user (2026-07-19):

- **D1 — Draft phase is a fully mutable working set.** Until sealed, ADRs can
  be edited and deleted through the CLI; no digest chain or frozen-field guard
  applies. `constitution seal` writes the manifest baseline and flips the log
  to append-only forever.
- **D2 — The goal statement renders into every project's `constitution.md`**
  as a short fixed preamble (plus spec §1 and skill texts). Existing repos
  pick it up on next `regen`.
- **D3 — One ADR can carry rules in multiple categories** (re-prioritized
  from backlog to must-change on 2026-07-14).

Made by the agent, flagged for review (all reversible before implementation):

- **D4 — No backward compatibility** (user, 2026-07-21): pre-v0.2 logs get no
  migration tooling and no compat grammar; wherever dropping compat
  simplifies, drop it. The only adopters are in-house (harness, the primitive
  itself; kafka-dq is being rebuilt anyway).
- **D5 — The rule is the unit of change** (user, 2026-07-21). The section is
  named `## Rules`; it holds one or more `### <category>` subsections, each
  holding one or more `#### <slug>` rule entries. Every rule is individually
  addressable as **`ADR-NNNN/<category>/<slug>`** (ADR-scoped identity —
  slug uniqueness required only within its category subsection; sealed
  bodies are frozen, so identity is immutable by construction). Later ADRs
  can retire specific prior rules without touching their files; whole-ADR
  supersede/deprecate stays for fully-dead decisions. This dissolves the old
  "superseding one rule supersedes the whole record" limitation.
- **A1 — One uniform Rules grammar.** The h3/h4 structure above is the
  *only* form — no plain-body Rules section, no untagged rules (text
  directly under `## Rules` or under a `### <category>` outside a `####`
  entry is a validation error). A single-rule ADR simply has one category
  with one entry. Every category tag must be in the configured vocabulary;
  slugs are short kebab identifiers.
- **A2 — Frontmatter `category:` is removed from the ADR schema.** With
  rules carrying their own categories there is no second source of category
  truth. Record-only ADRs (no `## Rules`) have no category at all — they
  never project, so they never needed one. `adr new --category` goes away;
  the founding-file and `adr new` flows supply categories via the Rules
  subsections themselves (`--principle` either learns a `<category>: <rule>`
  prefix form or is dropped in favor of founding-file-only — implementer's
  choice, it's a convenience flag).
- **A6 — Retirement is declared in the amending ADR's frontmatter**:
  optional `supersedes-rules:` / `removes-rules:` lists of
  `ADR-NNNN/<category>/<slug>` refs (`supersedes-rules` when this ADR's own
  rules replace the retired ones, `removes-rules` when a constraint simply
  stops applying — identical fold effect, the verb documents intent).
  Frontmatter over inline heading annotations because it is machine-checkable
  event metadata in one place, symmetric with the existing `supersedes:`
  field, and pure removals have no new rule to hang an inline annotation on.
- **A7 — Resurrection semantics.** The fold honors retirement directives
  only from *currently accepted* ADRs. Superseding an amender therefore
  resurrects what it had retired — the projection stays a pure function of
  the active set; a superseding ADR must re-retire anything it does not want
  back. `regen` warns when a supersede resurrects rules; the gov skill
  documents the behavior.
- **A3 — `phase:` is a required field in `constitution.yml`.**
  `constitution init` writes `phase: draft`; sealing is always an explicit,
  human-approved act. No absent-field defaulting logic (D4). Consent policy
  is orthogonal and still gates every draft-phase write (`--approve` for
  non-TTY agent shells).
- **A4 — Founding is a staged process, stated in the skill** (user's 07-14
  shape): (1) purpose + very-high-level design ADRs → (2) research the
  technology bets → (3) technical-architecture ADRs informed by that
  research → seal when the bets are validated. The init skill stops selling
  finality; it ends by saying the constitution stays in draft until sealed.
- **A5 — Milestone order** M1 → M4 below (multi-category first: self-contained
  and backward compatible; phase/seal second; skills third; docs/ADRs woven
  through).

## The changes

### 1. Purposeful projection (D2)

`internal/render/template.go` gains a fixed preamble after `# Constitution`:

> The source of truth for this project's standing technical decisions — how
> recurring problems are solved (architecture, mapping, testing, process) —
> so that requirements can stay functional and need not re-explain
> implementation choices.

Also lands in spec §1 (purpose), `constitution-gov` (framing when it
force-inlines the constitution), and `constitution-init` (the interview's
opening frame: we are capturing the "how" so requirements stay the "what").
Golden files regenerate; determinism/replay guarantees unchanged.

### 2. Rules grammar: multi-category, multi-rule, per-rule retirement (D3–D5, A1, A2, A6, A7)

The canonical shape:

```markdown
## Rules

### architecture

#### hex-core
Structure the service as hexagonal (ports and adapters); the domain
core imports no framework or adapter types.

#### explicit-boundary-mapping
Boundary mapping is explicit per adapter; never share domain types
across the wire.

### testing

#### three-tier-tests
Three tiers: per-class unit, domain-with-fakes, integration via
Testcontainers.
```

and a later amendment:

```yaml
supersedes-rules: [ADR-0002/testing/three-tier-tests]
removes-rules: [ADR-0002/architecture/explicit-boundary-mapping]
```

- Schema: `category` leaves the frontmatter (meta parse, conditional
  validation, and the `--category` flag all delete — net code removal);
  optional `supersedes-rules:` / `removes-rules:` ref lists arrive.
- Parser: the Rules section parses as h3 categories / h4 slug entries;
  validates slug format (kebab), slug uniqueness within each category
  subsection, and rejects any untagged text (leading prose under `## Rules`
  or a `### <category>`).
- Fold/renderer: builds the ref → rule map over accepted ADRs, subtracts
  refs retired by currently-accepted ADRs (A7), then buckets surviving
  rules into `CategorySection`s; section order stays `constitution.yml`
  vocabulary order; within a section, rules sort by ADR id then in-file
  order, each cited `ADR-NNNN · date`. (Whether the rendered rule heading
  is the slug or the ADR title is an M1 rendering detail.)
- Validation: every `###` tag must be in the vocabulary; every retirement
  ref must resolve to a rule in an earlier accepted ADR; double-retiring an
  already-retired ref is an error; a supersede that resurrects rules (A7)
  produces a warning naming them.
- The retired ADR's file is **never touched** — retirement is fold-time
  masking, so the guard, manifest, and replay machinery are unaffected.
- `renumber` learns to rewrite the `ADR-NNNN/` prefix inside rule refs
  (same class of rewrite it already does for `supersedes:`).
- `--founding-file` grammar documents the h3/h4 form.
- Acceptance: one ADR with rules in `purpose` + `architecture` + `testing`
  + `process` renders four populated sections from one record; two rules in
  one category render as two entries; an amending ADR retires one rule of a
  three-rule ADR and the projection drops exactly that rule; superseding
  the amender resurrects it (with the warning); untagged text and dangling
  refs are rejected with precise errors; `regen` + the from-empty replay
  guard pass on a log exercising all of the above. (Byte-identity with
  pre-v0.2 renders is explicitly a non-goal — D4.)
- One-time in-house migration, by hand: the harness constitution's 3 ADRs
  and this repo's own log get their `category:` moved into a
  `### <cat>` / `#### <slug>` entry (or, for record-only ADRs, simply
  dropped), then re-baselined. No tooling.

### 3. Founding phase: draft → seal (D1, A3)

- `constitution.yml` gains a required `phase: draft | sealed` (A3, D4 — no
  defaulting; pre-v0.2 configs are invalid until the one-line field is
  added, which the in-house migration does by hand).
- New commands, all consent-gated: `adr edit <id>` (revise body, title,
  category, rules), `adr rm <id>`, `constitution seal`. `edit`/`rm` refuse
  when sealed, pointing at supersede/deprecate instead.
- Guard becomes phase-aware: in draft it validates parse + vocabulary only
  (no digest chain, no frozen-field checks — this dissolves the retro's
  "authorized re-baseline" and mid-reset false-alarm friction); `seal` writes
  the manifest baseline and full guard semantics begin there.
- `seal` re-renders, reports rule-length warnings one final time as a
  pre-seal review checklist, and tells the user this is the last cheap edit.
- Both kafka-dq wipes replay as: `adr edit 0002` (drop the welded tech bet) /
  a handful of `adr edit`/`adr new` — no `rm -rf`, no guard violations, no
  history theater.

### 4. Skills rewrite (A4 + retro items 2–5 + 07-19 notes)

`constitution-init` becomes an explicitly **interactive, multi-turn, staged**
founding process:

- **Elicitation contract up front:** the skill MUST interview and MUST NOT
  write the founding file until the human confirms the content — the
  anti-formalize-before-elicit clause (same root cause as the
  `lifecycle-refine` finding; fixed here for this primitive).
- **Opinionated question catalog** replacing the bare starter-list offer.
  Each question maps to a category and to concrete config where applicable:
  - How do we track issues? (→ `sourceTracking`, process rules)
  - What agents do we build for? (→ `--target` AGENTS.md / CLAUDE.md / both)
  - Do we do spec-driven development, with which framework? How do we manage
    our spec? (→ process rules; e.g. spec-lifecycle adoption)
  - What's the project structure? (→ architecture rules)
  - How do we solve recurring problems — mapping, testing, error handling,
    persistence? (→ the "how" rules the goal statement demands; a rule may
    delegate to a skill, e.g. "hexagonal per kentra-skills:java-hexagonal")
  - Proposed revised starter vocabulary (review item):
    `purpose, architecture, code-style, testing, process, tooling, security, data`
    (adds `purpose` + `tooling` to today's list).
- **Rules-vs-bets triage:** for every founding principle, a third question
  beyond rule-vs-record — *validated, or an assumption research could kill?*
  Unvalidated bets become record-only ADRs or a parked list, never clauses
  inside a rule-bearing structural ADR; revisited in stage 3 after research.
- **Front-loaded projection model:** before category selection, explain
  sections = categories and how multi-category rules fan out (wipe #1 was
  purely this expectation gap).
- **Staged flow + draft framing (A4):** init ends in draft phase with the
  three-stage roadmap and seal criteria stated.
- **Consent mechanics documented** (init + adr-draft + gov): under `strict`
  in a non-TTY shell the interactive `[y/N]` always fails; `--approve` with
  the harness permission prompt as the human gate is the sanctioned path.
- `adr-draft` gains the **settledness gate**: if the decision emerged from
  brainstorming or a sketch, confirm it is settled before drafting. Not-yet-
  settled signal words: "brainstorm", "thoughts?", "how could we",
  "something like".
- All edits fan out via the managed skill trees (regeneration is part of any
  skill edit).

### 5. Minor CLI hygiene (retro item 6)

- Rule-length warning fires once, at the offending ADR's creation/edit (and
  once more in the `seal` review), not on every subsequent write.
- Guard's mid-operation "working tree ahead of HEAD" reporting is largely
  dissolved by draft phase; verify the sealed-phase message distinguishes
  in-progress CLI writes from genuine tamper.

## Milestones

1. **M1 — Rules grammar + per-rule fold** (schema simplification, h3/h4
   parser, retirement/resurrection fold, validation, goldens, replay-guard
   proof). Self-contained; breaking by design (D4).
2. **M2 — draft/seal phase** (`phase` config, `adr edit`/`adr rm`/`seal`,
   phase-aware guard, init defaults to draft).
3. **M3 — skills rewrite** (init staged interview + question catalog +
   triage + consent mechanics; adr-draft settledness gate; gov preamble).
4. **M4 — docs + self-governance**: spec + implementation-plan updated
   (binding-docs discipline), preamble template change, and the primitive's
   own ADR log records these decisions (D1–D3 at minimum) via its own flow.

## Constraints carried forward

- Byte-deterministic projection and the from-empty replay guard are
  invariants; every renderer/parser change re-proves them (ADR-0011 stack).
  Determinism is untouched by D4 — what's dropped is compatibility with
  *old* output, not reproducibility of the new.
- Sealed-phase append-only semantics are exactly today's semantics — v0.2
  adds a phase *before* them, it relaxes nothing after `seal`.
- Neutral mechanism: nothing here brands the primitive; the question catalog
  names kentra tools only as examples.
