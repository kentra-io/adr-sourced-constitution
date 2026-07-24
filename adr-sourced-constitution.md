# `adr-sourced-constitution` — Design Specification

*Generated: 2026-07-01. Reframed 2026-07-02 as a **standalone, general-purpose SDD primitive** (was: a Spec-Kit-bound harness module). Status: **IMPLEMENTED — v0.1 shipped, v0.2 M1–M4 shipped 2026-07-23.** Kept as the original design document. Companion to [planning.md](./planning.md) (§6b, §7, §8), [mvp-plan.md](./mvp-plan.md) (Phase 1), [observability.md](./observability.md).*

> **What this document is.** The buildable design of **`adr-sourced-constitution`** — a standalone primitive that models a project's governing "constitution" (its principles + accumulated architectural decisions — the *HOW* of the project) as an **event-sourced projection of an immutable ADR log**, deterministically rendered to a plain `constitution.md` that **any** SDD tool can consume. It ships as a **Go CLI + agent-agnostic skills + thin per-framework adapters**, integrating by default via a dedicated folder + an agent-instructions pointer. Produced through a structured grill/brainstorm + a cross-framework deep-research run (2026-07-01/02). Every decision carries its rationale; deferred items are flagged, not dropped.

---

## 0. Terminology (locked)

| Term | Meaning |
|---|---|
| **ADR** | Architecture Decision Record. The atomic, MADR-compliant **event** in the constitution's log. One decision per record. |
| **constitution** | The projection rendered from the **active** ADR set — the governed "HOW". A single plain file, `constitution.md`. |
| **feature-spec** | A per-issue / per-feature spec (previously "spec"). |
| **living-spec** | A projection synthesized from **multiple** feature-specs (+ ADRs) — the descriptive "what the system is". A **separate** module, out of scope here. |
| **founding principles** | The ADRs seeded at bootstrap by `constitution init`, using the reserved `bootstrap` source-ref. |
| **source-ref** | The pluggable per-ADR reference to its originating spec/issue; its **format is configured per project** (§7). Absent when spec-tracking = `none`. |

> **Later:** evaluate the [ubiquitous-language skill](https://www.skills.sh/mattpocock/skills/ubiquitous-language) for terminology consistency (deferred — §12).

---

## 1. Purpose & scope

**Purpose.** The constitution is the **"HOW" of a project** — how we build things: its principles and accumulated architectural decisions. It is the artifact that warrants the **most human attention** in planning. It is a **tool-neutral governance artifact**: `constitution.md` is a plain Markdown file (crisp MUST/SHOULD rules preferred, descriptive prose allowed) consultable by *any* planning tool. Used three ways (§8): **(a) planning support** — loaded into a planning agent's context to shape technical design and how functional/non-functional requirements are met (the primary, proactive use); **(b) a plan-validation gate**; **(c) code validation** (a deferred background drift sweep).

> **v0.2 (landed M1, 2026-07-21):** this goal statement is no longer just prose in this spec — it is rendered **verbatim as a fixed preamble** under `# Constitution` at the top of every generated `constitution.md` (proposal D2), so any agent opening the file (pointer-followed or not) sees what the document is for before it sees a single rule. See §6.

**This is a STANDALONE, general-purpose primitive** — not a harness-internal module and not bound to any one SDD framework. The research (§14) confirmed the gap is real: **none** of Spec-Kit, OpenSpec, or superpowers stores governance as an immutable append-only ADR log that deterministically projects the rules file, and **none has a native ADR/decision-log concept at all**. It has **no hard dependency** on the harness's feature-spec pipeline, engine, or observability plane; the only couplings (spec-tracking, planning tool, consent policy) are *configured* at adoption (§7).

**Dual purpose of each ADR record.** Every ADR serves both (i) **building the constitution** (as an event in the projected log) and (ii) **being a proper, standalone MADR ADR in the repo** (browsable, recognizable, interoperable with existing ADR tooling). This dual purpose drives two schema decisions below: MADR compliance (§4.1) and a mutable `status` field (§5).

**In scope:** the ADR log + supersede/deprecate semantics; the deterministic `constitution.md` projection; `constitution init` bootstrap/adoption; the governance skill; the plan-validation gate (emits `deviation.json`). **Out of scope:** the living-spec projection (separate module); the staged feature-spec pipeline; hard enforcement inside Conductor (this primitive *emits records + provides checks*; an engine/CI *blocks* — §5.4, §8b).

---

## 2. Architecture — event-sourcing (Model A)

The constitution is **not an authored document**. It is a **projection** of an event log:

```
   Human ⇄ agent conversation      (paste a rule, or ask the agent to draft/modify one)
     │  human accepts  ── the accept IS the write ──
     ▼
   constitution/adr/ADR-0007-*.md   ← the EVENT.  A proper MADR ADR.  Append-only log.  Source of truth.
     │  regen  (deterministic: read all ADRs → take active set → group by category → render)
     ▼
   constitution/constitution.md     ← the PROJECTION.  Regenerated, committed, never hand-edited.
     │  loaded / validated-against by any planning tool
     ▼
   planning support · plan-validation gate · code-drift sweep   (§8)
```

**Load-bearing consequences:**
- **The ADR log is the sole source of truth.** State is reconstructed by **replaying the ADR files**, never by diffing `constitution.md`'s git history. Git stores/versions the files; it is not the query interface.
- **Every governance change is an ADR.** No direct edit path to the constitution. Add / change / retire a rule = append an ADR (retire = a superseding or deprecating ADR — §5).
- **Authoring happens at ADR-acceptance time, not at projection time.** By the time `regen` runs, every decision is already human-accepted. The projection is a faithful *render*, never a creative act — this is what guarantees the constitution cannot drift from its decisions.

> **Why not "edit constitution.md, use git history as the log"?** A git diff is unstructured — a line change, not a decision. It can't be cited by the gate (`violates ADR-0007` is a governance primitive; a commit SHA is not), carries no Context/rationale/category/source-ref, and has no supersede semantics. Event-sourcing exists so the events are first-class and addressable.

---

## 3. The three layers

The primitive is deliberately layered so the core is reusable and the framework coupling is thin.

```
 Layer 1 — CORE (Go single binary `constitution`)         deterministic engine; no LLM, no framework
   constitution init | adr new | adr edit | adr rm | adr renumber | supersede | deprecate | seal | regen | guard | deviation validate
        ▲
 Layer 2 — AGENT SURFACE (agent-agnostic skills)               conversational + semantic; wraps the CLI
   constitution-init interview · propose/draft ADR · the plan-validation gate (emits deviation.json)
        ▲
 Layer 3 — INTEGRATIONS
   • DEFAULT (zero-framework): a dedicated  constitution/  folder + a managed pointer in the
     project's agent-instructions file(s) (AGENTS.md and/or CLAUDE.md) → any agent loads it
   • FRAMEWORK ADAPTERS (thin): Spec-Kit hook · OpenSpec context/schema · superpowers skill
   • PLUGGABLE SEAMS: spec-tracking source-ref format · consent policy
```

- **Layer 1 (Go CLI)** owns everything deterministic: parse ADR frontmatter/body, resolve the supersede/deprecate graph, render `constitution.md`, run the immutability guard. Chosen Go for a **single static zero-dependency binary** (best cross-framework adoption; bakes into claudebox via `COPY`) distributed by **GoReleaser → Homebrew tap + GitHub Releases** (+ `go install`). See §10.
- **Layer 2 (skills)** owns the conversational and *semantic* work the CLI can't: the `constitution-init` interview, drafting an ADR from conversation, and the **plan-validation gate** (which reasons about a plan vs the constitution and writes `deviation.json`). Shipped agent-agnostic as **skills only** — no duplicate slash-command files. *(Errata, 2026-07-02: Claude Code merged slash commands into skills, `.claude/commands/` is legacy; Codex deprecated `~/.codex/prompts/` likewise; Gemini CLI natively supports `SKILL.md`. A skill *is* the command.)*
- **Layer 3 (integrations)** — the default is framework-free (§7.1); framework adapters (§9) are thin glue on top.

---

## 4. ADR — the record

### 4.1 Schema (minimal-MADR-compliant)

Adopts **MADR v4** (MIT+CC0) — the recognized ADR convention — for interop and the dual-purpose goal (§1). **Minimal-MADR-compliant: 0 mandatory sections missing.** Markdown with YAML frontmatter + MADR body headings:

> **v0.2 (M1, 2026-07-21 — proposal D3/D5/A1/A2, supersedes the schema originally shown here):** `category` is **removed from frontmatter** — a record-only ADR has no category at all, because category is now a property of a *rule*, not of the ADR that carries it (one ADR may hold rules in several categories, §4.2). The single optional `## Rule` section (plan §2.12) is replaced by an optional `## Rules` section with its own two-level grammar. Two new optional frontmatter fields retire individual prior rules. Current schema:

```markdown
---
id: ADR-0007
title: Prefer composition over inheritance for domain services
date: 2026-07-01
status: accepted                 # accepted | superseded | deprecated  (see §5)
source: FS-0042                  # source-ref — format per configured tracker (§7); omit if tracking=none
supersedes: ADR-0003              # optional; present only when this ADR supersedes another
supersedes-rules: [ADR-0002/testing/old-tiers]  # optional; rule refs this ADR retires with a replacement
removes-rules: [ADR-0002/testing/no-mutation]   # optional; rule refs this ADR retires with NO replacement
---

## Context and Problem Statement
<why this decision arose>

## Considered Options            # MANDATORY per MADR v4 — may be a single bullet for principle-style rules
<options weighed>

## Decision Outcome
<the decision / choice and its rationale — the durable record>

## Consequences
<tradeoffs, follow-ons>          # optional in MADR; we keep it

## Rules                          # OPTIONAL — present ⇒ rule-bearing ⇒ projects (§6)
### architecture                  # a "### <category>" subsection per category this ADR contributes rules to
#### hex-core                     # a "#### <slug>" entry per rule in that category (kebab-case, unique within it)
<the standing rule, 1–3 lines — the text the projection renders verbatim>

#### explicit-boundary-mapping
<another rule, same category>

### testing                       # a second category, same ADR — multi-category is the point of the grammar
#### three-tier-tests
<a rule filed under `testing` instead>
```

- **MADR-derived body** (headings match MADR verbatim): `Context and Problem Statement` (req), `Considered Options` (**required by MADR v4** — one of its three mandatory body sections; keep it in every ADR, even as a single bullet for principle-style rules), `Decision Outcome` (req), `Consequences` (optional in MADR; we keep it). *(Corrected 2026-07-02 against the MADR v4 templates — an earlier revision mislabeled `Considered Options` as optional.)*
- **Rule-bearing marker — one custom optional section, now with an internal grammar** (proposal D5/A1, v0.2 M1 — supersedes the single-`## Rule` design of plan §2.12): an ADR body MAY include one `## Rules` section. Its content is strict two-level Markdown: every rule lives under exactly one `### <category>` heading and one `#### <slug>` heading; the slug's body is 1–3 lines of prose (the rule text, rendered verbatim). `category` and `slug` are both required to be lowercase kebab-case (`[a-z0-9]+(-[a-z0-9]+)*`); a category with zero rule entries, a slug repeated within its category, an empty rule body, or *any* untagged text anywhere in the section (before the first `### `, between a `### ` and its first `#### `, or a heading deeper than h4) is a validation error — never silently dropped. One ADR MAY carry rules in **several** categories (a category subsection per category it touches); the whole ADR is rule-bearing (and projects) iff it carries at least one rule anywhere in the section. An ADR with no `## Rules` section is a catalog-only record that stays in the log alone. The `Decision Outcome` remains mandatory but **does not project** — the projection renders only `## Rules` content (§6).
- **Rule identity.** Each rule is individually addressable as **`ADR-NNNN/<category>/<slug>`** (a *rule ref*) — the slug is unique only within its own `### <category>` subsection of its own ADR, so the full triple is globally unambiguous. Refs are frozen once the ADR is accepted (and, post-M2, once the log is sealed — §5) exactly like the rest of the body; they are the addressing scheme the retirement mechanism below cites.
- **Per-rule retirement** (proposal A6, v0.2 M1): an ADR's frontmatter MAY list `supersedes-rules:` and/or `removes-rules:` — each a list of rule refs the ADR retires. Both have the identical fold effect (the ref stops projecting, §6); the two verbs exist only to record *intent* (replaced-by-something-in-this-ADR vs. dropped-with-nothing-replacing-it), mirroring the ADR-level `supersede`/`deprecate` split (§5.2) at rule granularity. A ref may retire only a rule of a **strictly earlier** ADR (no forward or self references); a ref that does not resolve to any rule anywhere in the log, or that two currently-accepted ADRs both retire, is a validation error. **Resurrection** (proposal A7): retirement only counts from ADRs that are themselves currently `accepted` — if the ADR that retired a rule is later itself superseded, that retirement stops applying and the rule projects again, unless the *new* superseding ADR re-retires it. `regen` warns (never blocks) on every such resurrection so it is never a silent surprise.
- **Frontmatter beyond MADR** (all MADR frontmatter is *optional*, so this stays compliant): `id`, `source`, `supersedes`, `supersedes-rules`, `removes-rules` are our additions; `status` uses MADR's own field. `category` was a frontmatter addition in v1; **removed in v0.2** (categories now live on rules, §4.2). We omit MADR's optional `decision-makers`/`consulted`/`informed` (YAGNI).
- **`status`** is a first-class field (§5) — restored so each record is a proper ADR. `proposed`/`rejected` never appear in the store (proposals are ephemeral — §5.1).
- **Deliberate deviations from MADR convention** (content-compliant; stated so they read as decisions, not oversights): (i) we use `status: superseded` + a separate derived `superseded-by:` field, where MADR's own worked example embeds the link in the status string (`status: superseded by ADR-0123`) — chosen for cleaner machine parsing in the event-sourced model; (ii) `date` means **date created** (frozen with the rest of the record), whereas MADR defines it as "last updated" — matches the immutability invariant (§5) without widening the guard's permitted mutations; (iii) filenames are `ADR-NNNN-slug.md` under `constitution/adr/`, not MADR's bare `NNNN-slug.md` in `docs/decisions/` — chosen for readability; stock MADR-ecosystem tools glob `^\d{4}-` and won't auto-discover our files without custom configuration (a known, accepted interop gap).

### 4.2 Category vocabulary
`category` is drawn from a **per-project vocabulary the author defines**, recorded in `constitution.yml` in the order sections are rendered. `constitution init` **proposes a reference starter list** as a *suggestion only*; the author trims/extends it. Once set, the vocabulary is governed — a new category is introduced by an ordinary ADR passing `--new-category` on the write that first uses it (implementation-plan §2.5; this resolves what was an open spike — §13).

> **v0.2:** category is now a property of **each rule**, not of the ADR (§4.1) — one ADR's rules can span several category subsections, each fanning into its own `constitution.md` section. The vocabulary mechanism itself (governed via an ordinary ADR, no distinct meta-record) is unchanged. The proposed starter list is now `purpose, architecture, code-style, testing, process, tooling, security, data` (`purpose` and `tooling` added, M3) — verify against `starterCategories` in `cmd/constitution/init.go` if this list drifts.

### 4.3 File layout (default, framework-free)
```
constitution/
  adr/
    ADR-0001-*.md          ← the append-only log; monotonic zero-padded ids
    ADR-0002-*.md
    .manifest.sha256       ← sealed phase only (§5); the tamper-evidence baseline guard checks
  constitution.md          ← the projection (regenerated; the file every tool reads)
constitution.yml           ← repo root; schemaVersion, phase, categories, consent, sourceTracking, skills (§7)
```
A top-level `constitution/` folder by default; **adapters map it into framework paths** (e.g. Spec-Kit's `.specify/memory/constitution.md`) — §9.

> **v0.2 (M2):** `.manifest.sha256` exists **only once the log is sealed** — a draft-phase repo has no manifest at all (`regen` deletes a stale one if present, so a crash between draft edits and a later `seal` always converges); `constitution seal` writes the manifest as its baselining step (§5). `constitution.yml` also gained the required `phase:` field (§5).

---

## 5. Immutability & the ADR lifecycle

The invariant: **an accepted ADR's *content* never changes; only its `status` may transition.** This is the *canonical* ADR model (Azure Well-Architected, adr-tools: the status line is the one recognized mutable exception). It serves the dual-purpose goal — a superseded ADR's raw file reads `status: superseded`, so the file is a faithful standalone ADR.

> **v0.2 (M2, 2026-07-23 — proposal D1/A3): the founding-phase model.** The invariant above is now the **sealed-phase** semantics — unchanged from everything §5.1–§5.4 originally described. A project's `constitution.yml` carries a required `phase: draft | sealed` field with **no default** (a missing field is a hard error naming a migration hint — defaulting either way would silently pick an immutability model for the author). The two phases:
> - **`draft`** — the founding working set. The log is **fully mutable through the CLI** (never by hand): besides append (`adr new`, `supersede`, `deprecate`), draft phase adds **`adr edit <id>`** (per-facet replace — `--title` renames the file, `--body-file` replaces the whole body including whether a `## Rules` section is present at all, `--rule` replaces only the `## Rules` section, the retirement-ref flags replace their list wholesale, an explicit empty value clears one) and **`adr rm <id>`** (deletes a record outright; refused while another ADR's `supersedes-rules`/`removes-rules` cites it, or while a later ADR supersedes it — that successor must be `rm`'d first, which as a side effect restores the original to `accepted`; ids are never renumbered, so removal leaves a permanent gap). Every draft mutation is still consent-gated exactly like `adr new` (§7.1) — draft mutability is a *relaxation of the frozen-content rule*, not of consent. There is **no manifest** in draft (§4.3) and the guard runs a reduced check set (§5.3).
> - **`sealed`** — reached only via **`constitution seal`** (consent-gated): it runs the full rule-length/200-line/resurrection warning checklist over the whole log, writes the manifest baseline, flips `phase: sealed`, and regenerates. From that point on the pre-v0.2 §5.1–§5.4 text below applies **exactly as originally written, with no further relaxation** — `adr edit`/`adr rm` refuse (pointing at `supersede`/`deprecate` instead), and sealing is irreversible by design (append-only forever after).
>
> The rationale (proposal §1, dogfooded against two kafka-dq false starts): a founding log is not yet a governed constraint — early ADRs are often welded-in technology bets that need to be revised or dropped cheaply before anything downstream depends on them. Freezing on the *first* accepted ADR punished exactly the iteration a founding phase needs; sealing is the explicit, human-approved moment a project commits to append-only governance. `init` always writes `phase: draft` (§7).

### 5.1 Append-by-construction
Proposals are **ephemeral** — they live in the agent conversation, never in `adr/`. A file lands in `adr/` **only when the human accepts it**, written with `status: accepted`. So `proposed`/`rejected` never enter the store, and the directory is append-only by construction. (Prior art: IETF RFCs — drafts ephemeral, published RFCs immutable; PR-gated ADRs where merge = acceptance.)

### 5.2 The only permitted mutation: status transition
Post-acceptance, the sole allowed change is `accepted → superseded | deprecated`, and it is **CLI-mediated**:
- **`constitution supersede ADR-0003`** — writes a *new* ADR (with `supersedes: ADR-0003`) **and** flips ADR-0003's `status` to `superseded` (+ a derived `superseded-by` back-link). Both actions atomic.
- **`constitution deprecate ADR-0003`** — retire a rule with **no** replacement (a gap pure supersede can't express): flips status to `deprecated`.
Body and all other frontmatter remain frozen forever.

> **v0.2:** this remains the whole story once **sealed**. In **draft**, `supersede`/`deprecate` still work exactly as above (retiring a rule can also be expressed at finer grain via `supersedes-rules`/`removes-rules`, §4.1), but the log additionally accepts `adr edit`/`adr rm` — see the phase-model note under §5.

### 5.3 The guard — field-scoped
Because the only legal change to an existing file is its status line, the immutability guard is: **new files allowed; existing `adr/` files may change only the `status` (and derived back-link) line — body + other frontmatter frozen.** (This is a step up from a trivial "added-only" diff; the CLI owning transitions keeps it controlled.) `constitution.md` is excluded from this path so the projection rewrites freely.

> **v0.2 (M2): the guard is phase-aware.** It now loads `constitution.yml` and always runs id-uniqueness; in **draft** it additionally checks the category vocabulary (a rule citing a category outside `constitution.yml`'s list is reported as violation kind `unknown_category`) and stops there — no git-history legality check and no manifest cross-check run before a project has sealed, since neither concept has meaning yet (there is no manifest, and the frozen-body guarantee this section describes doesn't start until sealing). Passing an explicit `--base`/`--merge-base` in draft is a usage error (exit 2: "git legality checks do not apply before `constitution seal`"). In **sealed** phase the field-scoped body/frontmatter check above and the manifest cross-check (§5.4) both run exactly as originally specified — sealing adds enforcement, it never removes any.

### 5.4 Enforcement placement (phased)
Local git hooks are **not** enforcement — same trust domain as the editing agent (bypassable, not installed in fresh boxes). Real enforcement sits **outside the agent**:
- **Phase 1 (no engine):** `constitution guard` runs in a skill/CI and **surfaces** a violation; human honors it.
- **Phase 2 (engine/CI):** the same `constitution guard` runs as a **Conductor gate step / required CI check** and **hard-blocks**. Optional hardening: a committed SHA-256 content manifest + branch protection on `constitution/adr/`. *(Errata, 2026-07-02: the manifest's write-path and an advisory cross-check are pulled forward into v1 — cheap, and the only guard mode that works without git; branch-protection wiring and tamper-evidence remain Phase 2. See implementation-plan §2.7.)*

> **v0.2 (M2):** the manifest described here is written once, at `constitution seal` (§5, §4.3) — not from the first accepted ADR. A draft-phase repo therefore has no manifest and `guard` cross-checks nothing against one (§5.3); the "Phase 1/Phase 2" enforcement-*placement* framing (skill/CI vs. engine-hard-block) is orthogonal to the founding phase and unchanged by it.

---

## 6. The constitution projection — deterministic render

`constitution.md` is produced by the **Layer-1 Go CLI** (`constitution regen`) — **deterministic, no LLM**:
1. Read all ADR files.
2. Take the **active set** = ADRs with `status: accepted`.
3. Fold per-rule retirement over the *whole* log (any status, since a retirement ref can address frozen history): index every rule of every ADR by its ref (`ADR-NNNN/<category>/<slug>`), then mask out every ref retired by an ADR that is **currently accepted** (`supersedes-rules`/`removes-rules`, §4.1). A ref retired only by an ADR that is *itself* no longer accepted is **not** masked — it resurrects — and `regen` emits a warning naming the resurrecting rule and its lapsed retirer (proposal A7); the superseding ADR that caused the lapse must re-retire it if that resurfacing is unwanted.
4. Group the surviving rules of the active set by `category` (§4.2), in the project's configured vocabulary order; within a category, order by (ADR numeric id, then file order of the rule within that ADR). A category with no surviving rule is omitted. Catalog-only ADRs (no `## Rules` section) and fully-masked ADRs contribute nothing.
5. Render: a fixed generated-file header, then `# Constitution`, then the **fixed purpose preamble** (§1 — "The source of truth for this project's standing technical decisions…", proposal D2, verbatim and unconditional — it appears even on the empty-constitution placeholder), then each category section (`## <category>`) with one `### <slug>` entry per rule: the rule's text **verbatim**, followed by a metadata line (`ADR-0007 · 2026-07-01 · source FS-0042`, the source segment omitted when absent). The `Decision Outcome` never projects — only `## Rules` content does.
6. Write the single `constitution.md`. If the active set (post-fold) carries **no** rule anywhere, write the placeholder form: header + preamble + one line — `No standing rules yet. Decision log: constitution/adr/.`

**`regen` runs automatically** as the final step of `adr new` / `adr edit` / `adr rm` / `supersede` / `deprecate` / `seal` (append-then-project, atomic), and is available standalone. The constitution is never stale.

> **v0.2 (M1, proposal D2/D5/A6/A7 — supersedes the single-category-per-ADR description originally here):** the unit the fold operates on moved from *ADR* to *rule*. One ADR's rules can land in several category sections at once; retirement is per-rule via frontmatter refs rather than only via a whole-ADR `supersede`/`deprecate`; and every render carries the fixed preamble regardless of how many rules exist. `adr new`/`supersede`/`edit` preflight the fold **before** the consent gate (dangling/forward/self/double-retire refs are rejected before anything is written, never discovered only at the next `regen`) — see §7.1/§8.

**Why deterministic (not agent-synthesized) for the constitution:** preserves the event-sourcing guarantee (a faithful render can't drift; LLM synthesis can); the gate needs stable 1:1 rule↔`ADR-id` citations; nothing is left to author (the human already authored each ADR); reproducible, cheap, testable. Prior art for render-from-ADR-log: adr-tools `generate toc` — but that's a flat link index; **our projection of the *active set* with derived status is the novel combination** (§14).

> **Contrast — the living-spec** (separate module) *is* agent-synthesized (merging many feature-specs is inherently generative). Deterministic constitution vs. synthesized living-spec: different tasks, different mechanisms, different modules.

---

## 7. `constitution init` — the adoption flow

Adoption is **one command**. `constitution init` runs a greenfield interview (wrapping the familiar "agent interviews you, drafts for approval" pattern) that both **configures the integration** and **seeds the founding principles**. Brownfield extraction is **deferred** (§12).

It gathers and records (into a project config file):
1. **Agent-instructions target(s)** — which file(s) get the managed pointer: `AGENTS.md` (cross-tool standard) and/or `CLAUDE.md` (**required for Claude Code, which does not natively read AGENTS.md** — §9.1).
2. **Planning-tool integration** — `none` (default folder only) | `spec-kit` | `openspec` | `superpowers` → selects the Layer-3 adapter (§9).
3. **Spec-tracking system** — configures the `source-ref` format: `none` (no `source` field) | the harness feature-spec | GitHub Issues | Jira | … (e.g. `FS-0042`, `#123`, `PROJ-45`). Founding ADRs use the reserved `bootstrap` source.
4. **Consent policy** (§7.1).
5. **Category vocabulary** (§4.2) — proposes a reference list; author defines their own.

Then it **seeds founding-principle ADRs** into `adr/` (not a hand-written `constitution.md` — the constitution is a projection) and runs `regen` to render the initial `constitution.md`, and the resulting `constitution.yml` carries **`phase: draft`** (§5) — founding is never entered already sealed.

> **v0.2 (M2/M3 — proposal A2/A4, supersedes the `--principle` flag and the "ends settled" framing originally here):** founding principles are supplied **only** via `--founding-file` — the flat `--principle "text"` flag is gone, because expressing per-principle categories/rules needs the file form. **Founding-file grammar:** one principle per `## <title>` heading (the heading becomes the ADR title; the text under it becomes the `Decision Outcome`); a `## Rules` heading immediately following a principle carries that principle's standing rules in the same `### <category>` / `#### <slug>` grammar as `adr new --rule` (§4.1) — a principle with no `## Rules` seeds a catalog-only record. Every founding ADR is pre-flighted (composed → parsed → fold-checked) before any write, exactly like `adr new` (§6).
>
> `init` also frames founding as **staged, not one-shot** (the `constitution-init` skill, Layer 2): (1) purpose + very-high-level design ADRs now, (2) research the technology bets before they harden into rule-bearing architecture ADRs, (3) technical-architecture ADRs informed by that research, then `constitution seal` once the bets are validated. The skill's elicitation contract runs a **rules-vs-bets triage** per candidate principle — standing rule / point-in-time record / **unvalidated bet** — so an assumption research could still kill never gets welded into a rule-bearing structural ADR; bets land as record-only ADRs or a parked list instead. `init` ends the interview **in draft**, explicitly: the log stays a cheap, `adr edit`/`adr rm`-able working set until the human runs `constitution seal` (§5).

### 7.1 Consent policy (configured, not hardcoded)
Whether/how strictly ADR acceptance requires human consent is a **project-owner decision at init**, not a law baked into the primitive (it is standalone and publishable). Recorded in config; applies to every acceptance.
- **In *our* harness projects: a HARD RULE** — no ADR accepted without explicit human consent; the agent may propose, only the human approves. Delivered as an **architectural checkpoint outside the agent's discretion** (a mandatory hook), because in-agent "governance by convention" can be subverted.
- **Other projects** may choose looser (advisory / category-scoped / batched / off). Policy vocabulary **TBD** (§13).

> **As built:** every mutating verb (`adr new`, `adr edit`, `adr rm`, `supersede`, `deprecate`, `renumber`, `seal`) takes `--approve`, the documented non-interactive path when `strict` (the shipped default) and stdin is not a TTY — the human still approves, just at the harness's own Bash-permission prompt rather than an interactive `[y/N]` (implementation-plan §2.4). Skills never pre-grant these commands in `allowed-tools`, so that prompt is the actual checkpoint regardless of consent policy.

### 7.2 The governance skill
A `SKILL.md` (Layer 2) every planning/execution agent loads. It codifies the governed-set rules: what the constitution/ADRs are, that they are append-only, how an agent must consult the constitution before proposing a plan, and that amendments follow the configured consent policy. Modeled on Anthropic's own constitution style (priority hierarchy + explain-the-why, CC0). It *prompts*; it does not *enforce* (enforcement is the hook/engine/CI). This skill is also the mechanism for use (a), planning support (§8).

---

## 8. Governance — how the constitution is used

Tool-neutral: `constitution.md` is a plain file; every consumer reads the same file, only the *mechanism* differs. Three uses:

**(a) Planning support — input to design (primary, proactive).** The planning agent has the constitution **on hand** as it designs; it shapes the technical design and how functional/non-functional requirements are met — informing the design as it is created, not merely checking it after. *Mechanism:* loaded into context via the governance skill (§7.2) / the agent-instructions pointer (§9.1).

**(b) Plan-validation gate.** A plan (from any planning tool) is validated against the constitution **before code exists**.
- *Mechanism:* a Layer-2 skill/adapter reasons about the plan vs the constitution and serializes findings to a machine-readable **`deviation.json`**, each **citing the specific violated `ADR-id`**. Because `constitution.md` now carries only rule-bearing active ADRs (§6, plan §2.12), a citation must name a **rule-bearing active** ADR — the CLI validator rejects a citation to a record-only ADR (it never appears in the constitution). In our harness the Spec-Kit adapter builds on `/speckit.analyze`; other tools consume `constitution.md` their own way.
- *Seam:* the primitive **emits** `deviation.json`; an engine/CI **enforces**. *Phasing:* Phase 1 surfaces + human honors; Phase 2 Conductor/CI hard-blocks.

**(c) Code validation — background drift sweep (DEFERRED).** A background process scans the **codebase** for drift from the constitution and files todos/issues that re-enter the pipeline. First-class to what the constitution is *for*, but **deferred** (§12); no framework offers a native seam for it — it's a standalone-CLI concern.

**Amendment loop.** A flagged deviation from (b)/(c) → *conform* (revise plan / fix code) or *amend* — subject to the consent policy §7.1. In our projects: **HARD RULE — no amendment without explicit human consent.**

> **v0.2: amendment is phase- and grain-aware.** Which mechanism amends a rule depends on where the project is and what's being changed:
> - **Draft phase** — the cheapest path: `adr edit` to revise a still-accepted founding ADR in place (§5), or `adr rm` to drop one outright (with its automatic supersede-undo heal case, §5). Appropriate while a rule is still a bet, not yet a governance constraint anyone downstream depends on.
> - **Sealed phase, whole-ADR grain** — `supersede`/`deprecate` (§5.2), unchanged from v1: a new ADR replaces or retires everything the old one established.
> - **Sealed phase, single-rule grain** — a `supersedes-rules`/`removes-rules` frontmatter entry on a new ADR (§4.1/§6) retires just the one rule ref, leaving the rest of the retiring — or retired-from — ADR's other rules and record content untouched. This is the mechanism proposal D3/D5 introduced specifically so a multi-category, multi-rule ADR doesn't need a whole-ADR supersede to amend one line of it.
>
> All three remain **consent-gated** identically (§7.1) — the phase/grain choice only changes *which* CLI verb records the amendment, never whether a human approved it.

---

## 9. Integrations

### 9.1 Default — zero-framework (the general-purpose adapter)
The baseline needs **no SDD framework**: the `constitution/` folder (§4.3) + a **managed pointer** the CLI writes/maintains in the project's agent-instructions file(s) — a block instructing the agent to read `constitution/constitution.md` before planning. Covers use (a) universally.
- **AGENTS.md** = the emerging cross-tool standard (Cursor, Codex, …). **CLAUDE.md** is also written because **Claude Code does not natively read AGENTS.md** (issue #6235). `init` asks which target(s) apply.
- **Spike:** do agents reliably *follow* a pointer, or must the constitution be *inlined*? (§13).

### 9.2 Framework adapters (thin) — verified seams (§14)
| Framework | Current governance | Adapter | Fit |
|---|---|---|---|
| **Spec-Kit** | Authored **mutable** `constitution.md` at hardcoded `.specify/memory/constitution.md`; placeholder-token template | A **mandatory `after_constitution` hook** (`.specify/extensions.yml`) runs `constitution regen` to that path; `plan.md`/`analyze.md` read it unchanged | ⚠️ **Worst philosophical fit** — its UX is hand-editing an authored doc; documented overwrite bugs (#1541/#1229) |
| **OpenSpec** | **No** constitution/ADR concept; governance = mutable `config.yaml` `context` + per-artifact `rules` | Inject the projected constitution into the **`config.yaml` context** block (reaches every artifact), and/or ship a **schema** (`openspec/schemas/<name>/`) | ✅ Clean complement (unmet gap) |
| **superpowers** | **No** constitution/ADR; implicit Philosophy baked into skills; per-feature docs only | A project **`SKILL.md`** that loads the constitution into planning context / runs the gate | ✅ Clean complement (fills the cross-feature gap) |

> **Irony worth noting:** Spec-Kit — the framework the harness is built on — is the *worst* philosophical fit (authored-mutable model fights a never-hand-edited projection). OpenSpec and superpowers are the *cleanest*. Not a blocker; sharpens the Spec-Kit overwrite spike (§13).

### 9.3 Honest reuse boundary
Domain logic (ADR log, supersede/deprecate, deterministic projection, guard, `deviation.json`, consent) is **~100% custom** — no tool provides it. What we reuse is **convention + plumbing**: MADR v4 (format), and, *per adapter*, each framework's context-loading/extension mechanism. `/speckit.analyze` and `/speckit.constitution` are prompt-patterns to crib, not commands to invoke.

---

## 10. Packaging & distribution

- **One primitive, one repo** — `adr-sourced-constitution` (public). The ADR log is the core's storage, **not** a separate product. (Supersedes the earlier two-extension `kentra-adr` + `…-constitution` split.)
- **Layer 1: a Go single static binary** (`constitution`), `CGO_ENABLED=0`, cross-compiled linux/macos/windows × amd64/arm64. Distributed via **GoReleaser → Homebrew tap + GitHub Releases** (+ `go install`); **baked into claudebox** via `COPY` (no runtime). Version-pinned (no Docker wrapper — a containerized CLI would add nested-container + bind-mount friction for a hot-path tool).
- **Layer 2:** agent-agnostic skills (no separate slash-command files — a skill *is* the command, §3) shipped in the same repo.
- **Layer 3:** the default folder/pointer integration + per-framework adapters (each thin).
- **License: MIT** — matches the broad-adoption goal and MADR/Spec-Kit precedent.
- **Standalone-first rollout:** ship the primitive; prove value harness-internal (Spec-Kit adapter first); then offer the OpenSpec + superpowers adapters as thin glue. Consider proposing a MADR-v4 + projection **convention** upstream before OpenSpec ships native ADRs (§13).

---

## 11. Component inventory

| Component | Layer | Role |
|---|---|---|
| `constitution` CLI | 1 (Go) | deterministic engine: `init`, `adr new`, `adr edit`, `adr rm`, `adr renumber`, `supersede`, `deprecate`, `seal`, `regen`, `guard`, `deviation validate` |
| ADR record + schema | 1 | minimal-MADR-compliant; mutable `status`; derived `superseded-by`; per-rule `## Rules` grammar + retirement refs (§4.1) |
| founding phase | 1 | `constitution.yml` required `phase: draft \| sealed`; draft = mutable working set, sealed = the pre-v0.2 immutability model (§5) |
| immutability guard | 1 | phase-aware: draft = parse + id-uniqueness + vocabulary; sealed = the original field-scoped check + manifest cross-check (§5.3) |
| projection (`regen`) | 1 | deterministic per-rule fold + render of the active set → `constitution.md`, fixed purpose preamble (§1/§6) |
| `constitution-init` interview | 2 (skill) | staged greenfield adoption: configure integration + seed founding ADRs, ends in draft (§7) |
| ADR draft/propose | 2 (skill) | draft an ADR from conversation for human acceptance; settledness gate; rule-vs-record-vs-bet triage |
| plan-validation gate | 2 (skill/adapter) | reason about plan vs constitution → emit `deviation.json` citing `ADR-id` |
| governance `SKILL.md` | 2 | governed-set rules; loads constitution into planning context (use a) |
| default folder + pointer | 3 | `constitution/` + managed AGENTS.md/CLAUDE.md block |
| Spec-Kit / OpenSpec / superpowers adapters | 3 | thin per-framework glue (§9.2) |
| spec-tracking + consent seams | 3 | configured at `init` (§7) |

*(Errata, 2026-07-02: `adr renumber` added — the id-collision escape hatch: a colliding id is by definition not yet merged into the shared log, so renumbering is a safe rename + frontmatter id edit. See implementation-plan §2.6. **v0.2 (M2):** `renumber` additionally refuses when any other ADR's `supersedes-rules`/`removes-rules` cites the old id. **v0.2 (M1/M2):** `adr edit`, `adr rm`, `seal`, and `deviation validate` added — see §5 and implementation-plan §4 for the current verb-by-verb behavior.)*

---

## 12. Deferred — explicitly not in MVP

| Item | Why |
|---|---|
| **Async drift detector** (codebase-vs-constitution sweep, use c) | Background worker; the plan gate covers MVP governance. |
| **Brownfield constitution extraction** | Greenfield only in v1; `init` interviews from scratch. |
| **ADR-log rollup / snapshot** | Event-sourcing snapshot to avoid replaying the full log; MVP replays all ADRs each regen. |
| **Agent-synthesized constitution prose** | MVP renders deterministically; synthesis risks drift. |
| **Ubiquitous-language skill integration** | Evaluate later for terminology consistency. |
| **Code-time constitution check** (diff-vs-constitution at execution) | Lives in the execution domain (planning.md §8 gate #2), not this primitive. |

---

## 13. Open items — build-time spikes (not blockers)

1. **Pointer reliability** — do agents reliably *follow* an AGENTS.md/CLAUDE.md pointer to `constitution.md`, or must it be *inlined*? *(Errata, 2026-07-02: softened per implementation-plan §1.5 — pointer reliability is load-bearing for the **UX quality** of use (a), not a correctness guarantee; Anthropic's own docs state CLAUDE.md content is context, not enforcement. The correctness backstop is the plan-validation gate (§8b), which re-reads `constitution.md` regardless of whether the agent followed the pointer.)* Still open as of v0.2 — unaffected by the rules-grammar/phase work.
2. **Spec-Kit `after_constitution` overwrite** — can the hook deterministically overwrite `constitution.md` without the agent re-injecting placeholder tokens, given the overwrite bugs (#1541/#1229)? Still open; framework adapters remain out of scope (implementation-plan §0 P1).
3. **source-ref pluggable contract** — concrete schema mapping across Spec-Kit specs / OpenSpec changes / GitHub issues / none. Still open; unaffected by v0.2.
4. **OpenSpec native-ADR collision** — #557/#721 may land native ADRs; first-mover case to propose a MADR-v4 + projection convention upstream? Still open; unaffected by v0.2.
5. **Consent-policy vocabulary** — strict / advisory / category-scoped / batched / off (§7.1). Still open as of v0.2 — the shipped vocabulary is still exactly `strict | off` (§7.1).
6. **RESOLVED (pre-v0.2, implementation-plan §2.5; reconfirmed unchanged by v0.2) — Category-vocabulary governance:** new category via an ordinary ADR, not a distinct meta-record — `adr new`/`adr edit`/`init`'s `--new-category` flag. v0.2 moved *what* carries a category (a rule, not the whole ADR, §4.1/§4.2) but not *how* the vocabulary itself is governed.
7. **`constitution guard` in CI vs Conductor** — where each adopter wires the Phase-2 hard block (§5.4). Still open as of v0.2 — orthogonal to the phase model (§5.3's phase-awareness is about *what* draft-vs-sealed guard checks, not *where* guard is wired in).

---

## 14. Research provenance

- **Grill/brainstorm session (2026-07-01/02)** over [planning.md §6b/§7/§8](./planning.md) + [mvp-plan.md](./mvp-plan.md). Key decisions: Model A event-sourcing · one ADR kind · **restored mutable `status`** + minimal-MADR compliance (added optional `Considered Options`, MADR heading names) · **Option 2 immutability** (append-by-construction + status-only mutation + field-scoped guard) · deterministic constitution vs synthesized living-spec · **standalone general primitive** (3-layer: Go CLI + skills + integrations) · default AGENTS.md/folder integration · pluggable spec-tracking + consent · MIT.
- **Five parallel Spec-Kit ecosystem research agents** (per-extension surveys) + a **file-immutability-enforcement** research agent (established: local hooks ≠ enforcement; CI/orchestrator gate is the real seam; SHA-256 manifest for tamper-evidence).
- **Cross-framework deep-research run (2026-07-02, 93 agents, 21 verified / 4 killed claims).** Findings: all three frameworks have a governing-rules concept but **none** stores it as an immutable ADR-log projection, and **none has a native ADR concept**; concrete seams verified (Spec-Kit `after_constitution` hook → hardcoded `.specify/memory/constitution.md`; OpenSpec `config.yaml` context / schemas; superpowers `SKILL.md`); Spec-Kit is the sharpest philosophical mismatch (authored-mutable). Novelty: ADR mechanics are canonical (Azure/AWS/Nygard); render-from-log has prior art (adr-tools `generate toc`); **"constitution as a projection of the active ADR set" is the novel combination**. Sources incl. github/spec-kit `templates/commands/{constitution,analyze,plan}.md`, Fission-AI/OpenSpec `docs/customization.md` + issues #447/#557/#721, obra/superpowers, adr/madr, Azure Well-Architected ADR guidance.
- **Correction to planning.md §6b:** the extensions §6b cited as patterns — `spec-validate`/`approval-state.json`, `architecture-guard`, `Mneme HQ` — **could not be verified to exist**; treat as misremembered. Also: Spec-Kit bare command aliases are unsupported; `init --force` clobbers core files. *(planning.md §6b/§7 to be synced — companion-sync debt, mirroring [observability.md §7](./observability.md).)*
