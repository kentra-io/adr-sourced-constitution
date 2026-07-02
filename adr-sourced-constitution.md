# `adr-sourced-constitution` — Design Specification

*Generated: 2026-07-01. Reframed 2026-07-02 as a **standalone, general-purpose SDD primitive** (was: a Spec-Kit-bound harness module). Status: **DESIGN — pending user review.** Companion to [planning.md](./planning.md) (§6b, §7, §8), [mvp-plan.md](./mvp-plan.md) (Phase 1), [observability.md](./observability.md).*

> **What this document is.** The buildable design of **`adr-sourced-constitution`** — a standalone primitive that models a project's governing "constitution" (its principles + accumulated architectural decisions — the *HOW* of the project) as an **event-sourced projection of an immutable ADR log**, deterministically rendered to a plain `constitution.md` that **any** SDD tool can consume. It ships as a **Go CLI + agent-agnostic skills/commands + thin per-framework adapters**, integrating by default via a dedicated folder + an agent-instructions pointer. Produced through a structured grill/brainstorm + a cross-framework deep-research run (2026-07-01/02). Every decision carries its rationale; deferred items are flagged, not dropped.

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
   constitution init | adr new | supersede | deprecate | regen | guard
        ▲
 Layer 2 — AGENT SURFACE (agent-agnostic skills + commands)   conversational + semantic; wraps the CLI
   constitution-init interview · propose/draft ADR · the plan-validation gate (emits deviation.json)
        ▲
 Layer 3 — INTEGRATIONS
   • DEFAULT (zero-framework): a dedicated  constitution/  folder + a managed pointer in the
     project's agent-instructions file(s) (AGENTS.md and/or CLAUDE.md) → any agent loads it
   • FRAMEWORK ADAPTERS (thin): Spec-Kit hook · OpenSpec context/schema · superpowers skill
   • PLUGGABLE SEAMS: spec-tracking source-ref format · consent policy
```

- **Layer 1 (Go CLI)** owns everything deterministic: parse ADR frontmatter/body, resolve the supersede/deprecate graph, render `constitution.md`, run the immutability guard. Chosen Go for a **single static zero-dependency binary** (best cross-framework adoption; bakes into claudebox via `COPY`) distributed by **GoReleaser → Homebrew tap + GitHub Releases** (+ `go install`). See §10.
- **Layer 2 (skills/commands)** owns the conversational and *semantic* work the CLI can't: the `constitution-init` interview, drafting an ADR from conversation, and the **plan-validation gate** (which reasons about a plan vs the constitution and writes `deviation.json`). Shipped agent-agnostic (skills *and* slash commands).
- **Layer 3 (integrations)** — the default is framework-free (§7.1); framework adapters (§9) are thin glue on top.

---

## 4. ADR — the record

### 4.1 Schema (minimal-MADR-compliant)

Adopts **MADR v4** (MIT+CC0) — the recognized ADR convention — for interop and the dual-purpose goal (§1). **Minimal-MADR-compliant: 0 mandatory sections missing.** Markdown with YAML frontmatter + MADR body headings:

```markdown
---
id: ADR-0007
title: Prefer composition over inheritance for domain services
category: architecture           # drives the projection section (§4.2)
date: 2026-07-01
status: accepted                 # accepted | superseded | deprecated  (see §5)
source: FS-0042                  # source-ref — format per configured tracker (§7); omit if tracking=none
supersedes: ADR-0003             # optional; present only when this ADR supersedes another
---

## Context and Problem Statement
<why this decision arose>

## Considered Options            # OPTIONAL — usually empty for principle-style rules; kept for MADR compliance
<options weighed, if any>

## Decision Outcome
<the rule / choice — what the projection renders>

## Consequences
<tradeoffs, follow-ons>          # optional in MADR; we keep it
```

- **MADR-derived body** (headings match MADR verbatim): `Context and Problem Statement` (req), `Considered Options` (**optional** — usually empty; present so each file is a legitimate standalone MADR ADR, per the dual-purpose goal), `Decision Outcome` (req), `Consequences`.
- **Frontmatter beyond MADR** (all MADR frontmatter is *optional*, so this stays compliant): `id`, `category`, `source`, `supersedes` are our additions; `status` uses MADR's own field. We omit MADR's optional `decision-makers`/`consulted`/`informed` (YAGNI).
- **`status`** is a first-class field (§5) — restored so each record is a proper ADR. `proposed`/`rejected` never appear in the store (proposals are ephemeral — §5.1).

### 4.2 Category vocabulary
`category` is drawn from a **per-project vocabulary the author defines**. `constitution init` **proposes a reference starter list** (TBD — e.g. `architecture`, `code-style`, `process`, `testing`, `security`, `data`) as a *suggestion only*; the author supplies their own. Once set, the vocabulary is governed — a new category is introduced by an ADR.

### 4.3 File layout (default, framework-free)
```
constitution/
  adr/
    ADR-0001-*.md        ← the append-only log; monotonic zero-padded ids
    ADR-0002-*.md
  constitution.md        ← the projection (regenerated; the file every tool reads)
```
A top-level `constitution/` folder by default; **adapters map it into framework paths** (e.g. Spec-Kit's `.specify/memory/constitution.md`) — §9.

---

## 5. Immutability & the ADR lifecycle

The invariant: **an accepted ADR's *content* never changes; only its `status` may transition.** This is the *canonical* ADR model (Azure Well-Architected, adr-tools: the status line is the one recognized mutable exception). It serves the dual-purpose goal — a superseded ADR's raw file reads `status: superseded`, so the file is a faithful standalone ADR.

### 5.1 Append-by-construction
Proposals are **ephemeral** — they live in the agent conversation, never in `adr/`. A file lands in `adr/` **only when the human accepts it**, written with `status: accepted`. So `proposed`/`rejected` never enter the store, and the directory is append-only by construction. (Prior art: IETF RFCs — drafts ephemeral, published RFCs immutable; PR-gated ADRs where merge = acceptance.)

### 5.2 The only permitted mutation: status transition
Post-acceptance, the sole allowed change is `accepted → superseded | deprecated`, and it is **CLI-mediated**:
- **`constitution supersede ADR-0003`** — writes a *new* ADR (with `supersedes: ADR-0003`) **and** flips ADR-0003's `status` to `superseded` (+ a derived `superseded-by` back-link). Both actions atomic.
- **`constitution deprecate ADR-0003`** — retire a rule with **no** replacement (a gap pure supersede can't express): flips status to `deprecated`.
Body and all other frontmatter remain frozen forever.

### 5.3 The guard — field-scoped
Because the only legal change to an existing file is its status line, the immutability guard is: **new files allowed; existing `adr/` files may change only the `status` (and derived back-link) line — body + other frontmatter frozen.** (This is a step up from a trivial "added-only" diff; the CLI owning transitions keeps it controlled.) `constitution.md` is excluded from this path so the projection rewrites freely.

### 5.4 Enforcement placement (phased)
Local git hooks are **not** enforcement — same trust domain as the editing agent (bypassable, not installed in fresh boxes). Real enforcement sits **outside the agent**:
- **Phase 1 (no engine):** `constitution guard` runs in a skill/CI and **surfaces** a violation; human honors it.
- **Phase 2 (engine/CI):** the same `constitution guard` runs as a **Conductor gate step / required CI check** and **hard-blocks**. Optional hardening: a committed SHA-256 content manifest + branch protection on `constitution/adr/`.

---

## 6. The constitution projection — deterministic render

`constitution.md` is produced by the **Layer-1 Go CLI** (`constitution regen`) — **deterministic, no LLM**:
1. Read all ADR files.
2. Take the **active set** = ADRs with `status: accepted` (drop `superseded`/`deprecated`).
3. Group by `category`.
4. Render each section from a fixed template: `title` + `Decision Outcome` body (+ `id`, `date`, `source`, and any derived `superseded-by`).
5. Write the single `constitution.md`.

**`regen` runs automatically** as the final step of `adr new` / `supersede` / `deprecate` (append-then-project, atomic), and is available standalone. The constitution is never stale.

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

Then it **seeds founding-principle ADRs** into `adr/` (not a hand-written `constitution.md` — the constitution is a projection) and runs `regen` to render the initial `constitution.md`.

### 7.1 Consent policy (configured, not hardcoded)
Whether/how strictly ADR acceptance requires human consent is a **project-owner decision at init**, not a law baked into the primitive (it is standalone and publishable). Recorded in config; applies to every acceptance.
- **In *our* harness projects: a HARD RULE** — no ADR accepted without explicit human consent; the agent may propose, only the human approves. Delivered as an **architectural checkpoint outside the agent's discretion** (a mandatory hook), because in-agent "governance by convention" can be subverted.
- **Other projects** may choose looser (advisory / category-scoped / batched / off). Policy vocabulary **TBD** (§13).

### 7.2 The governance skill
A `SKILL.md` (Layer 2) every planning/execution agent loads. It codifies the governed-set rules: what the constitution/ADRs are, that they are append-only, how an agent must consult the constitution before proposing a plan, and that amendments follow the configured consent policy. Modeled on Anthropic's own constitution style (priority hierarchy + explain-the-why, CC0). It *prompts*; it does not *enforce* (enforcement is the hook/engine/CI). This skill is also the mechanism for use (a), planning support (§8).

---

## 8. Governance — how the constitution is used

Tool-neutral: `constitution.md` is a plain file; every consumer reads the same file, only the *mechanism* differs. Three uses:

**(a) Planning support — input to design (primary, proactive).** The planning agent has the constitution **on hand** as it designs; it shapes the technical design and how functional/non-functional requirements are met — informing the design as it is created, not merely checking it after. *Mechanism:* loaded into context via the governance skill (§7.2) / the agent-instructions pointer (§9.1).

**(b) Plan-validation gate.** A plan (from any planning tool) is validated against the constitution **before code exists**.
- *Mechanism:* a Layer-2 skill/adapter reasons about the plan vs the constitution and serializes findings to a machine-readable **`deviation.json`**, each **citing the specific violated `ADR-id`**. In our harness the Spec-Kit adapter builds on `/speckit.analyze`; other tools consume `constitution.md` their own way.
- *Seam:* the primitive **emits** `deviation.json`; an engine/CI **enforces**. *Phasing:* Phase 1 surfaces + human honors; Phase 2 Conductor/CI hard-blocks.

**(c) Code validation — background drift sweep (DEFERRED).** A background process scans the **codebase** for drift from the constitution and files todos/issues that re-enter the pipeline. First-class to what the constitution is *for*, but **deferred** (§12); no framework offers a native seam for it — it's a standalone-CLI concern.

**Amendment loop.** A flagged deviation from (b)/(c) → *conform* (revise plan / fix code) or *amend* (append a new/superseding/deprecating ADR — subject to the consent policy §7.1). In our projects: **HARD RULE — no amendment without explicit human consent.**

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
- **Layer 2:** agent-agnostic skills + slash commands shipped in the same repo.
- **Layer 3:** the default folder/pointer integration + per-framework adapters (each thin).
- **License: MIT** — matches the broad-adoption goal and MADR/Spec-Kit precedent.
- **Standalone-first rollout:** ship the primitive; prove value harness-internal (Spec-Kit adapter first); then offer the OpenSpec + superpowers adapters as thin glue. Consider proposing a MADR-v4 + projection **convention** upstream before OpenSpec ships native ADRs (§13).

---

## 11. Component inventory

| Component | Layer | Role |
|---|---|---|
| `constitution` CLI | 1 (Go) | deterministic engine: `init`, `adr new`, `supersede`, `deprecate`, `regen`, `guard` |
| ADR record + schema | 1 | minimal-MADR-compliant; mutable `status`; derived `superseded-by` |
| immutability guard | 1 | field-scoped check (new-file OK; existing files status-line-only) |
| projection (`regen`) | 1 | deterministic render of the active set → `constitution.md` |
| `constitution-init` interview | 2 (skill) | greenfield adoption: configure integration + seed founding ADRs |
| ADR draft/propose | 2 (skill) | draft an ADR from conversation for human acceptance |
| plan-validation gate | 2 (skill/adapter) | reason about plan vs constitution → emit `deviation.json` citing `ADR-id` |
| governance `SKILL.md` | 2 | governed-set rules; loads constitution into planning context (use a) |
| default folder + pointer | 3 | `constitution/` + managed AGENTS.md/CLAUDE.md block |
| Spec-Kit / OpenSpec / superpowers adapters | 3 | thin per-framework glue (§9.2) |
| spec-tracking + consent seams | 3 | configured at `init` (§7) |

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

1. **Pointer reliability** — do agents reliably *follow* an AGENTS.md/CLAUDE.md pointer to `constitution.md`, or must it be *inlined*? (Load-bearing for universal planning support.)
2. **Spec-Kit `after_constitution` overwrite** — can the hook deterministically overwrite `constitution.md` without the agent re-injecting placeholder tokens, given the overwrite bugs (#1541/#1229)?
3. **source-ref pluggable contract** — concrete schema mapping across Spec-Kit specs / OpenSpec changes / GitHub issues / none.
4. **OpenSpec native-ADR collision** — #557/#721 may land native ADRs; first-mover case to propose a MADR-v4 + projection convention upstream?
5. **Consent-policy vocabulary** — strict / advisory / category-scoped / batched / off (§7.1).
6. **Category-vocabulary governance** — new category via an ordinary ADR vs a distinct meta-record (§4.2 assumes ordinary ADR).
7. **`constitution guard` in CI vs Conductor** — where each adopter wires the Phase-2 hard block (§5.4).

---

## 14. Research provenance

- **Grill/brainstorm session (2026-07-01/02)** over [planning.md §6b/§7/§8](./planning.md) + [mvp-plan.md](./mvp-plan.md). Key decisions: Model A event-sourcing · one ADR kind · **restored mutable `status`** + minimal-MADR compliance (added optional `Considered Options`, MADR heading names) · **Option 2 immutability** (append-by-construction + status-only mutation + field-scoped guard) · deterministic constitution vs synthesized living-spec · **standalone general primitive** (3-layer: Go CLI + skills + integrations) · default AGENTS.md/folder integration · pluggable spec-tracking + consent · MIT.
- **Five parallel Spec-Kit ecosystem research agents** (per-extension surveys) + a **file-immutability-enforcement** research agent (established: local hooks ≠ enforcement; CI/orchestrator gate is the real seam; SHA-256 manifest for tamper-evidence).
- **Cross-framework deep-research run (2026-07-02, 93 agents, 21 verified / 4 killed claims).** Findings: all three frameworks have a governing-rules concept but **none** stores it as an immutable ADR-log projection, and **none has a native ADR concept**; concrete seams verified (Spec-Kit `after_constitution` hook → hardcoded `.specify/memory/constitution.md`; OpenSpec `config.yaml` context / schemas; superpowers `SKILL.md`); Spec-Kit is the sharpest philosophical mismatch (authored-mutable). Novelty: ADR mechanics are canonical (Azure/AWS/Nygard); render-from-log has prior art (adr-tools `generate toc`); **"constitution as a projection of the active ADR set" is the novel combination**. Sources incl. github/spec-kit `templates/commands/{constitution,analyze,plan}.md`, Fission-AI/OpenSpec `docs/customization.md` + issues #447/#557/#721, obra/superpowers, adr/madr, Azure Well-Architected ADR guidance.
- **Correction to planning.md §6b:** the extensions §6b cited as patterns — `spec-validate`/`approval-state.json`, `architecture-guard`, `Mneme HQ` — **could not be verified to exist**; treat as misremembered. Also: Spec-Kit bare command aliases are unsupported; `init --force` clobbers core files. *(planning.md §6b/§7 to be synced — companion-sync debt, mirroring [observability.md §7](./observability.md).)*
