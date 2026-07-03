# `adr-sourced-constitution` — Implementation Plan (v1)

*Generated: 2026-07-02. Status: **PLAN — pending user review.** Companion to [adr-sourced-constitution.md](./adr-sourced-constitution.md) (the design spec) and, in the harness repo, [mvp-plan.md](../mvp-plan.md) (Phase 1) and [planning.md](../planning.md) (§7/§8). Produced from the spec plus a 6-topic parallel research run + completeness critique (2026-07-02, all framework/library claims verified against live primary sources — see §14).*

> **What this document is.** The sequenced, buildable plan for **v1** of the primitive. The spec decides *what it is*; this decides *what gets built, in what order, with which concrete stack*, pins every TBD the spec left open (§2), and records the spec corrections the research surfaced (§1). Milestones carry validation contracts (Definition of Done); nothing is complete without proving it.

---

## 0. v1 scope (locked with the user, 2026-07-02)

| # | Question | Decision |
|---|---|---|
| **P1** | Integration surface | **Core + default only**: Layer-1 Go CLI + Layer-2 skills + the zero-framework `constitution/` folder + managed pointer integration. **All framework adapters (Spec-Kit, OpenSpec, superpowers) are deferred** to a later plan. This also removes spec spikes §13.2 (Spec-Kit overwrite) and §13.4 (OpenSpec collision) from v1 — carried forward, not dropped. |
| **P2** | Distribution | **Full pipeline in v1**: GoReleaser → GitHub Releases + Homebrew tap, `go install` support, predictable linux artifact URL for claudebox `COPY`. |
| **P3** | Module path | **`github.com/kentra-io/adr-sourced-constitution`**. The `kentra-io` GitHub org exists; the repo does not yet (submodule has one local seed commit, no remote) — repo creation is Milestone 0. |
| **P4** | Spikes | Desk-researchable spikes were **resolved before this plan** (results baked in below); only live-agent/live-OS experiments remain scheduled (§10). |

Non-goals for v1 (inherited from spec §12 + P1): framework adapters, brownfield extraction, drift sweep, log rollup/snapshot, agent-synthesized prose, code-time check.

---

## 1. Spec errata — corrections to feed back into the spec

Research against primary sources found these defects in the spec text. They are **documentation fixes, not design changes** (the design survives all of them). **Items 1–3 were applied to the spec on 2026-07-02** (user-approved); items 4–7 land with the code they describe.

1. **§4.1 has MADR optionality backwards.** Per the actual MADR v4 templates (`adr/madr`, `template/adr-template*.md`): `Considered Options` is **mandatory** (one of only three mandatory body sections, with `Context and Problem Statement` and `Decision Outcome`); `Consequences` is the **optional** one. The spec labels `Considered Options` "OPTIONAL". Our template already includes both sections so we are compliant *today*, but the label is a trap — an implementer could drop `Considered Options` and silently break minimal-MADR compliance. Fix the comment/prose; keep the section always present (possibly a single bullet) even for terse principle-ADRs.
2. **MADR frontmatter confirmed all-optional** (the minimal template ships *no* frontmatter block at all) — spec §4.1's compliance claim holds. But note two deliberate deviations to document explicitly: (a) MADR's own convention embeds the forward link in the status string (`status: superseded by ADR-0123`); we use `status: superseded` + a separate derived `superseded-by:` field for machine parsing. (b) MADR defines `date` as "last updated"; we define it as **date created** (frozen forever) — this matches the immutability model and avoids adding `date` to the guard's permitted mutations.
3. **Filename/folder deviation to document:** canonical MADR is `NNNN-slug.md` in `docs/decisions/`; ours is `ADR-NNNN-slug.md` in `constitution/adr/`. Content-compliant, but stock MADR ecosystem tools glob `^\d{4}-` and won't auto-discover our files — a stated interop gap, chosen deliberately for readability.
4. **§3/§7.2 "skills *and* slash commands" is stale.** Claude Code merged slash commands into skills (changelog v2.1.3; `.claude/commands/` is legacy — a skill *is* the command). v1 ships **skills only**, no duplicate command files. Codex likewise deprecated `~/.codex/prompts/` in favor of skills; Gemini CLI's `.toml` commands are superseded by its native SKILL.md support.
5. **§13.1 "load-bearing" framing should be softened.** Anthropic's own docs state CLAUDE.md content is context, not enforcement. Pointer reliability is load-bearing for the *UX quality* of use (a); the *correctness* backstop is the plan-validation gate, which re-reads `constitution.md` regardless of whether the agent followed the pointer. (Resolution of the spike itself: §2.1.)
6. **§11 component inventory gains one verb:** `adr renumber` (§2.6, the id-collision escape hatch).
7. **§5.4's SHA-256 manifest is partially pulled forward** into v1 (write-path + advisory check; see §2.7) — a conscious scope amendment, since it is cheap and it is the only guard mode that works without git.
8. **§2.8's "founding ADRs use the reserved `bootstrap` source" applies only when `type != none`** — under `type: none` the "no ADR may carry a `source`" rule wins, so `init` seeds founding ADRs with no `source` field at all and only stamps `bootstrap` once source tracking is enabled (discovered at M5 review, where the sentence read as an unconditional stamp).

---

## 2. Pinned decisions (spec TBDs + gaps the critique surfaced)

Each was left open by the spec (§13) or unaddressed by the research briefs; pinned here so the build has no TBDs. All are assumptions the user can veto at review.

### 2.1 Pointer strategy (spec §13.1) — **hybrid, decided per-tool by the CLI, resolved**
- **CLAUDE.md → true inline** via `@constitution/constitution.md` inside the managed block. Verified against Anthropic's live memory docs: `@path` imports are "expanded and loaded into context at launch" (real inlining, max 4 hops — we use 1). This *resolves* the spike for Claude Code by construction. Claude Code still doesn't read AGENTS.md (issue #6235 open, no timeline), so writing CLAUDE.md remains required.
- **AGENTS.md → short textual pointer** ("Before planning, read `constitution/constitution.md`; it is this project's governing constitution and takes precedence over inferred conventions."). AGENTS.md is now a Linux Foundation (AAIF) standard with 20+ tool adopters; no cross-tool import primitive exists, and the available evidence says short, concrete, single-file pointers are the regime where pointers *are* followed.
- **Belt-and-braces:** the governance skill (§6) additionally force-inlines the constitution via dynamic context injection, independent of pointer compliance.
- **Size guardrail:** `regen` warns (not blocks) when `constitution.md` exceeds ~200 lines, per Anthropic's adherence guidance. A category-scoped/summarized variant is a post-v1 item.

### 2.2 Managed block format
Doctoc/terraform-docs/ansible-blockinfile pattern, **versioned**, HTML-comment markers (bonus: Claude Code strips block-level HTML comments before injection, so the marker lines cost zero context tokens):

```
<!-- BEGIN adr-sourced-constitution v1 (managed — do not edit by hand; `constitution init` updates it) -->
…pointer text or @import line…
<!-- END adr-sourced-constitution v1 -->
```

Semantics: locate by exact marker pair; rewrite only the interior; append a new block at EOF if absent. Store a hash of the last-written interior (in `constitution/.state`); if the interior drifted from what the CLI last wrote, show a diff and require confirm/`--force` (soft check — the user's file is not the ADR log). Marker text must stay **byte-stable across CLI versions** (documented ansible-blockinfile failure mode); the `v1` token is how a future pointer→inline strategy change migrates non-breakingly.

### 2.3 ADR write interface (Layer-2 → Layer-1 boundary; critique gap #1)
The spec requires writes be CLI-mediated ("the accept IS the write"), but an ADR body is multi-paragraph prose. Pinned design:

```
constitution adr new --title "…" --category architecture [--source FS-0042] --body-file <path|->
```

- `--body-file` (or stdin via `-`) supplies the **MADR body only** (the four `##` sections). The CLI validates shape (mandatory headings present, §1.1), allocates the id, composes frontmatter, writes atomically, auto-runs `regen`. The agent never writes into `adr/` directly.
- `supersede`/`deprecate` take the target id (+ `--body-file` for the superseding ADR's body).
- Draft flow: the Layer-2 skill drafts the body to a temp file during conversation; on human acceptance it invokes the CLI. Rejected drafts never touch `adr/` (append-by-construction preserved).

### 2.4 Consent policy (spec §13.5) — v1 vocabulary: `strict | off`
- **`strict`** (harness default): every mutating command (`adr new`, `supersede`, `deprecate`) requires explicit confirmation — interactive TTY confirm, or `--approve` for scripted use. **Honesty note, recorded here deliberately:** Layer 1 cannot verify a *human* typed it. The v1 architectural checkpoint is the **agent-harness permission boundary**: the shipped skills do NOT pre-grant `allowed-tools` for mutating `constitution` commands, so the human approves each write at the Bash-permission prompt — outside the agent's discretion, exactly as §7.1 demands. Hard enforcement beyond that (hooks/CI/engine) is Phase 2, per the spec's own phasing.
- **`off`**: no CLI-level gate (for adopters who want pure tooling).
- `advisory`/`category-scoped`/`batched` are **deferred** — no design exists and the harness's HARD RULE doesn't need them.

### 2.5 Category vocabulary governance (spec §13.6)
Vocabulary lives in `constitution.yml`. `adr new` **hard-errors** on an unknown category; introducing one requires `--new-category`, which (a) appends the category to config and (b) still produces an ordinary ADR (no meta-record type), matching the spec's default assumption. `init` proposes the reference starter list (`architecture`, `code-style`, `process`, `testing`, `security`, `data`) as a suggestion only.

### 2.6 ID allocation & collision strategy (spec §13.3 adjacent; guard brief)
- Keep `ADR-NNNN` sequential zero-padded ids; `adr new` scans `adr/` for the highest id (optimistic, same as adr-tools).
- Collisions are a **CI-time problem**: `guard` (merge-base mode) includes an id/filename-uniqueness check; adopters are told to enable "require branches up to date before merging" or a merge queue. (adr-tools' #102 lock-file idea was never shipped for good reason; log4brains' date-slug switch sacrifices readability we won't give up.)
- **`constitution adr renumber <old-id> <new-id>`** is the escape hatch — safe by construction: a colliding ADR is by definition not yet merged/accepted into the shared log, so nothing references it; renumber is a pure rename + frontmatter id edit.

### 2.7 Guard modes + manifest (spec §13.7 + §5.4)
- **Git mode (default):** shell out to the system `git` binary (`git diff --name-status`, `git show <ref>:<path>`) — *not* go-git, *not* hand-parsed unified diffs. The legality check is a **structured comparison**: parse frontmatter + body on both sides; allow-list only `status:` and `superseded-by:` to differ. Diff base: `HEAD` for local/skill checks; `git merge-base <target> HEAD` for CI (computed locally, not trusted from `pull_request.base.sha`); reference workflow uses `fetch-depth: 0`.
- **Manifest mode (no-git fallback, pulled forward):** `constitution/adr/.manifest.sha256` records each ADR's frozen-content hash (canonicalized: body + frozen frontmatter fields, LF-normalized — exact canonicalization rule is an M3 design note). Auto-rewritten by every mutating command; `guard` always cross-checks disk vs manifest. **Advisory in v1** — no branch-protection wiring, no tamper-evidence claim against a malicious committer (they can edit both files in one commit; that's Phase 2's branch protection).
- **Exit contract:** `0` clean · `1` violations found · `2` guard could not run. `--format json` emits a machine payload (violation `kind` enum: `frozen_field_changed | body_changed | file_deleted | file_renamed | manifest_mismatch | id_collision`, each citing the ADR id) — JSON-only on stdout, pipeable. `--format github` annotations: deferred nice-to-have.
- v1 ships `guard` + one **documented, optional, advisory** GitHub Actions example job. No required checks, no Conductor — Phase 1 "surfaces; human honors", per spec §5.4.

### 2.8 source-ref contract (spec §13.3)
Minimal typed config, no live tracker integration in v1:

```yaml
sourceTracking:
  type: none | generic | github-issue | jira   # none ⇒ no `source` field allowed
  pattern: '…'                                  # optional regex; defaults per type (#\d+, [A-Z]+-\d+)
```

CLI validates presence/shape of `source` when `type != none`; founding ADRs use the reserved `bootstrap` source. Harness feature-spec format arrives with the (deferred) Spec-Kit adapter.

### 2.9 deviation.json (spec §8b)
Written by the plan-validation skill via the CLI-validated path, default `./deviation.json` (override `--out`; in harness use the skill writes into the issue's spec-folder). **Not** under `constitution/` — it is per-plan output, not part of the log/projection domain. Schema: purpose-built but SARIF-shaped (future GitHub code-scanning upload stays a thin mapping) and reusing Spec-Kit `analyze`'s severity vocabulary (so the future adapter needs no translation):

```json
{
  "generatedAt": "…", "constitutionHash": "sha256-of-constitution.md", "plan": "path-or-ref",
  "deviations": [{
    "id": "D-001", "adrId": "ADR-0007", "severity": "CRITICAL|HIGH|MEDIUM|LOW",
    "rule": "ADR title", "location": {"file": "…", "lines": "12-40"},
    "summary": "…", "recommendation": "conform|amend", "recommendationDetail": "…"
  }],
  "summary": {"critical": 0, "high": 1, "medium": 0, "low": 0}
}
```

`adrId` is required on every deviation — the citation is the governance primitive.

### 2.10 Project config
`constitution.yml` at **repo root** (sibling of `constitution/` — config is neither log nor projection). Plain `yaml.Unmarshal` into a versioned struct (`schemaVersion: 1`; unknown version ⇒ refuse with a clear message — no migration machinery in v1). Fields: agent-instruction targets, consent policy, sourceTracking, category vocabulary. No Viper (multi-source merging is unneeded weight).

### 2.11 Dogfood decision
The `adr-sourced-constitution` repo **governs itself**: M5 runs `constitution init` on this repo, and the founding ADRs are the very decisions in this plan (§2.1–§2.10, stack choices in §3). First real-world exercise + living example in one.

### 2.12 Rule-bearing projection — constitution as curated read model (**revision, user-decided 2026-07-03**)
v1 as first built projected **every** accepted ADR into `constitution.md`. User review rejected that: the log is the catalog; the constitution is a concise rulebook of standing, planning-relevant rules. Many ADRs are point-in-time records that belong in the log only. Pinned design:

- **Opt-in marker = the rule text itself.** An ADR body MAY include one optional `## Rule` section: a 1–3 line normative statement of the standing rule the decision establishes. An ADR **with** a Rule section is *rule-bearing* and projects into `constitution.md`; one **without** is a catalog-only record. No separate boolean — presence is the flag, content is the projection. (New documented MADR deviation: one custom optional section.)
- **Projection content:** per rule-bearing active ADR: title heading, the **Rule section body verbatim**, metadata line. The Decision Outcome no longer projects. Categories with no rule-bearing active ADRs are omitted. If no active ADR is rule-bearing, render the generated-file header + `# Constitution` + one line: `No standing rules yet. Decision log: constitution/adr/.`
- **CLI:** `adr new --rule <text>` composes the section (canonical position: last body section). A `--body-file` MAY instead carry its own `## Rule` section; supplying both is an error; an empty/whitespace-only Rule section is invalid. `supersede` gains the same `--rule`/body-file semantics for the superseding ADR.
- **Immutability unchanged:** the Rule section is body — frozen at acceptance. Promoting a record to a rule, demoting, or rewording a rule = supersede. No new mutable frontmatter fields; **zero guard changes**.
- **Size guardrails:** `regen` warns (never blocks) when a Rule section exceeds 5 lines; the ~200-line whole-file warning stays.
- **deviation.json tightening:** `deviation validate` requires `adrId` to cite an active **rule-bearing** ADR (the gate reasons over `constitution.md`, which now contains only rules; citing a record-only ADR is a category error).
- **init:** each `--principle` yields a rule-bearing ADR (the principle text is the Rule). In `--founding-file`, a per-ADR `## Rule` section is honored; absent ⇒ record-only.
- **Skills:** `adr-draft` asks whether the decision is a standing rule or a point-in-time record, and drafts the Rule section accordingly; `plan-gate`/`constitution-gov` prose updated to the curated-projection model.
- **Migration:** none mechanically — pre-existing ADRs without Rule sections simply stop projecting. This repo's own constitution empties to the placeholder; the founding **re-seed is user-interactive with per-ADR approval** and is scheduled before the `v0.1.0` cut (also closes the consent gap flagged 2026-07-03).

---

## 3. Stack & repo layout (all choices verified live, 2026-07-02)

| Concern | Choice | Why (verified) |
|---|---|---|
| CLI framework | **urfave/cli v3** | Zero-dependency (stdlib only), actively maintained (v3.8+, June 2026). Cobra is fine but drags pflag; docgen/completion polish not needed for 7 flat verbs. |
| YAML | **`go.yaml.in/yaml/v3`** — everywhere, incl. guard | `gopkg.in/yaml.v3` archived ~Apr 2025; this is the official drop-in successor (yaml org), adopted by k8s sigs / Harbor / Forgejo. |
| Frontmatter | Manual `---` delimiter split + yaml on the block | `adrg/frontmatter` is decode-only and pinned to old yaml.v2. |
| Status mutation | **Line-targeted textual patch** (find `^status:` in the frontmatter block, replace value; insert `superseded-by:` right after) | No YAML lib guarantees byte-for-byte round-trip of untouched content (goccy AST editor has open newline bug #285). Only a raw-line edit satisfies §5.3 exactly. yaml parse is used to *validate*, never to rewrite an accepted ADR. |
| Rendering | stdlib `text/template`, fed a pre-sorted `[]CategorySection{Name; ADRs}` built in Go (categories in config order; ADRs by **numeric** id) | Nondeterminism risk is Go-side map iteration during grouping, not the template. Byte-for-byte golden-testable. |
| Atomic writes | Hand-rolled temp-file-in-same-dir + `os.Rename`; Windows via `MoveFileEx(REPLACE_EXISTING\|WRITE_THROUGH)` through `golang.org/x/sys/windows` | `google/renameio` exports **nothing on Windows** — disqualified since windows/amd64+arm64 are build targets. |
| Multi-file "transaction" | Ordered sequence: **write new ADR → patch old ADR status → regen last**; crash mid-sequence always leaves the log self-consistent, and `regen` re-derives the projection ("the log is truth" *is* the rollback strategy — no WAL) | Pure-projection property of the design; document, don't build transactions. |
| Go / CI | go.mod floor **1.25**, CI matrix {1.25, 1.26} (1.24 is EOL); golangci-lint **v2** via `golangci-lint-action@v9` (pin lint version with config — v1→v2 schema break); `actions/checkout@v6` + `actions/setup-go@v6` **uniformly** (briefs disagreed v4/v5 vs v6 — standardize on v6, re-verify once at M0) | go.dev release policy; golangci docs. |
| Testing | stdlib golden files (`testdata/`, `-update` flag) · **`rogpeppe/go-internal/testscript`** for black-box CLI e2e · native `go test -fuzz` on the frontmatter-split/status-patch code (seed corpus committed; seed-only per PR, scheduled `-fuzztime=5m` job) | All verified alive/current; zero extra test deps beyond go-internal. |

```
adr-sourced-constitution/
  cmd/constitution/main.go        ← REQUIRED location: `go install …/cmd/constitution@latest`
                                     derives the binary name from this dir ⇒ `constitution`
                                     (root main.go would install as `adr-sourced-constitution`)
  internal/
    adr/        parse, validate, schema (frontmatter split + yaml), id allocation
    patch/      line-targeted status/superseded-by editor  (fuzz target #1)
    render/     active-set resolution, grouping, text/template projection
    guard/      git-mode + manifest-mode checks, violation model, JSON output
    config/     constitution.yml load/validate (schemaVersion)
    scaffold/   init: folder, config, founding ADRs, managed blocks, skills fan-out
    atomicwrite/ temp+rename helper (unix/windows)
  skills/                          ← single-source Layer-2 bundles (open Agent-Skills layout),
    constitution-init/SKILL.md       go:embed'ed into the binary; also directly consumable
    adr-draft/SKILL.md               via `npx skills add kentra-io/adr-sourced-constitution`
    plan-gate/SKILL.md               (no dependency on that Node tooling ourselves)
    constitution-gov/SKILL.md
  templates/                       constitution.md.tmpl, ADR skeleton, pointer-block texts
  docs/                            CI example (advisory guard job), claudebox COPY snippet
  .goreleaser.yaml  .golangci.yml  .github/workflows/{ci,release}.yml
  adr-sourced-constitution.md  implementation-plan.md  README.md  LICENSE
  constitution/                    ← the repo's own (dogfood, M5)
```

**Version reporting:** ldflags-injected vars for GoReleaser builds; `runtime/debug.ReadBuildInfo()` fallback (module version + `vcs.revision`) so `go install` users still get a meaningful `constitution --version`.

---

## 4. CLI surface (v1 — 7 verbs)

| Command | Behavior |
|---|---|
| `constitution init` | Scaffold `constitution/{adr,}`, write `constitution.yml` (flags or interactive prompts; the *conversational* interview is the Layer-2 skill wrapping this), seed founding ADRs (`--founding-file` / repeated `--principle`, source `bootstrap`), write managed pointer blocks into chosen targets, fan out skills (§6), `regen`. Idempotent re-run; interior-drift requires confirm/`--force` (§2.2). |
| `constitution adr new` | §2.3. Validates category (§2.5) + source (§2.8) + body shape; allocates id; atomic write; consent gate (§2.4); auto-`regen`; manifest update. |
| `constitution supersede <id>` | New ADR (with `supersedes:`) + line-patch old ADR to `status: superseded` + `superseded-by:`; ordered per §3; auto-`regen`. |
| `constitution deprecate <id>` | Line-patch to `status: deprecated`; auto-`regen`. |
| `constitution adr renumber <old> <new>` | §2.6 escape hatch: rename + id-field edit; refuses if any ADR references `<old>`. |
| `constitution regen` | Deterministic projection (spec §6): read all → active set → group → render → atomic write. Warns >~200 lines (§2.1). Also refreshes managed blocks + manifest. |
| `constitution guard` | §2.7: `--base <ref>` / merge-base / manifest-only; exit 0/1/2; `--format json`. |

`constitution.md` render template (per **rule-bearing** active ADR under its category heading, §2.12): title as rule heading, the `Rule` section body, then a metadata line (`ADR-0007 · 2026-07-01 · source FS-0042`). Record-only ADRs stay in the log and do not project. A short generated header states the file is a projection and must never be hand-edited, pointing at `adr/`. Exact layout is finalized against golden fixtures. `adr new`/`supersede` accept `--rule <text>` (§2.12).

---

## 5. Default integration (Layer 3, zero-framework)

Written by `init`, refreshed by `regen`:
- **`CLAUDE.md`** managed block containing `@constitution/constitution.md` (direct import, *not* `@AGENTS.md`, so Claude's constitution load doesn't depend on unrelated AGENTS.md content).
- **`AGENTS.md`** managed block with the short pointer text (§2.1).
- `init` asks which target(s) apply; default **both** when both files exist or the user opts in.

---

## 6. Layer-2 skills (single-sourced in `skills/`, fanned out by `init`)

Fan-out targets (real copies, not symlinks — Windows + prior art from OpenSpec/Spec-Kit): `.claude/skills/<name>/` (Claude Code; no `.agents` alias support), `.agents/skills/<name>/` (covers **both** Codex CLI and Gemini CLI natively — verified discovery paths), `.cursor/skills/<name>/` (Cursor ≥2.4). No legacy command files anywhere (§1.4).

| Skill | Invocation stance | Key design |
|---|---|---|
| `constitution-init` | `disable-model-invocation: true` (explicit `/constitution-init`) | Conversational greenfield interview (targets, consent, tracking, categories, founding principles) → drives `constitution init` + `adr new` per accepted principle. Non-forked context (must converse). |
| `adr-draft` | Auto-invocable | Drafts a MADR body from conversation → temp file → on human acceptance calls `adr new`. **Does not pre-grant** mutating `constitution` commands in `allowed-tools` — the permission prompt is the consent checkpoint (§2.4). |
| `plan-gate` | `disable-model-invocation: true` | Reads plan + `constitution.md`, reasons rule-by-rule, emits `deviation.json` (§2.9) citing ADR ids; consider `context: fork` + read-only agent. Also runs `constitution guard` and folds violations in. |
| `constitution-gov` | Auto-invocable (this *is* use (a)) | Governed-set rules (append-only, consult-before-planning, amendment = ADR under consent policy), modeled on Anthropic's constitution style (priority hierarchy + explain-the-why). **Force-inlines the active constitution via dynamic context injection** (`` !`constitution cat` `` or `cat constitution/constitution.md`) — pointer-independence by construction. |

Authoring the actual prompt/rule text of `constitution-gov` and `plan-gate` is real design work — budgeted as its own milestone (M5), not an afterthought.

---

## 7. Milestones — each with a validation contract

Sequenced so every milestone is independently verifiable; M1–M3 are pure-Go and fast to iterate; agent-facing work (M5) starts only once the CLI contract is frozen.

### M0 — Repo bootstrap
Create `kentra-io/adr-sourced-constitution` (public), wire the submodule remote, push. `go.mod` (floor 1.25), `cmd/constitution` skeleton with `--version`, CI workflow (test matrix {1.25,1.26} × {linux,macos,windows} + separate lint job), `.golangci.yml` (v2 schema), testscript harness wired, apply spec errata (§1).
**DoD:** CI green on all matrix legs; `go install …/cmd/constitution@<sha>` produces a binary named `constitution` whose `--version` reports build info.

### M1 — Read path: parse → active set → projection
`internal/adr` (frontmatter split, schema validation, id parsing), `internal/config`, `internal/render`, `regen` command. Golden fixtures: a synthetic log (~12 ADRs across categories incl. superseded/deprecated chains) → byte-exact `constitution.md`. Fuzz seeds for the parser.
**DoD:** `regen` over fixtures is byte-identical across 100 runs and across OSes (CI-verified); malformed ADRs fail with precise errors (file, line, field); parser fuzz corpus runs clean.

### M2 — Write path: `adr new`, `supersede`, `deprecate`, `renumber`
`internal/patch` (line-targeted editor — fuzz target), `internal/atomicwrite`, id allocation, consent gate (`strict|off`), category/source validation, ordered multi-file sequence with `regen` last, manifest write-path.
**DoD (testscript):** full lifecycle e2e (init-less fixture → new → supersede → deprecate → renumber) passes on 3 OSes; **byte-preservation test** — after `supersede`, the old ADR differs from the original in *only* the `status:` and `superseded-by:` lines (asserted by diff); **crash-injection test** — killing the process between each pair of writes never leaves an invalid log, and a following `regen` converges; `strict` mode refuses without confirm/`--approve`.

### M3 — `guard` + manifest check
Git mode (HEAD + merge-base), manifest mode, id-uniqueness, exit codes, `--format json`, canonicalization design note (§2.7), example advisory CI job in `docs/`.
**DoD (testscript, against real scratch git repos):** each planted mutation class is caught and correctly typed — body edit, frozen-frontmatter edit, deletion, rename, duplicate id, out-of-band edit with git history rewritten (manifest catch); a status-only legal transition passes; JSON output validates against its schema; exit codes honored.

### M4 — `init` + default integration + skills fan-out
`internal/scaffold`: config authoring, founding-ADR seeding, managed blocks (markers, drift-hash, `--force`), `go:embed` skills fan-out to the three target trees, idempotency.
**DoD:** on an empty scratch repo: one `init` yields a working setup (`regen`/`guard` clean, pointer blocks present, skills in place); re-running `init` is a no-op (byte-identical tree); hand-editing inside a managed block is detected and requires `--force`; `CLAUDE.md` `@import` path resolves; targets honor the config's tool selection.

### M5 — Layer-2 skill content + dogfood + live-agent spike
Author the four SKILL.md bodies (§6); `deviation.json` end-to-end; **dogfood:** run `init` on this repo, seed founding ADRs from this plan's decisions (§2.11). Run the remaining live spike (§11.1) with a real Claude Code session (+ at least one AGENTS.md-only tool if available).
**DoD (live sessions, the Phase-1-style acceptance):** an agent proposes an ADR from conversation and the write happens **only** after explicit human approval at the permission prompt; a plan containing a **deliberately planted violation** yields `deviation.json` citing the correct ADR-id; the governance skill demonstrably has the constitution in context (probe question); this repo's own `constitution/` is live and `guard` runs in its CI.

### M5.5 — Rule-bearing projection (post-review revision, §2.12)
Implement §2.12 end to end: `## Rule` section parse/validate in `internal/adr`; rule-only projection + empty-constitution placeholder + 5-line Rule warning in `internal/render`/`regen`; `--rule` flag (and body-file Rule detection, both-is-error) on `adr new`/`supersede`; init `--principle`/`--founding-file` rule semantics; `deviation validate` rule-bearing tightening; skills prose updates (adr-draft rule-or-record question; plan-gate/constitution-gov alignment) with fan-out refresh; spec doc (`adr-sourced-constitution.md`) aligned to the curated-projection model; goldens/testscripts updated with mixed rule/record fixtures; dogfood `regen` (this repo's constitution.md shrinks to the placeholder pending the user-interactive re-seed).
**DoD:** goldens byte-exact incl. mixed rule/record logs and the empty-constitution case; testscript covers `--rule` vs body-file Rule vs both (error) vs record-only default; `deviation validate` rejects citations of record-only ADRs; determinism ×100 and coverage gates hold; dogfood CI green with the placeholder constitution.

### M6 — Distribution
`.goreleaser.yaml` (**`homebrew_casks`, not the removed `brews`**; `main: ./cmd/constitution`; CGO_ENABLED=0; linux/darwin/windows × amd64/arm64; `-trimpath -s -w` + version ldflags; sha256 checksums), create `kentra-io/homebrew-tap`, fine-grained PAT as `HOMEBREW_TAP_TOKEN` (default `GITHUB_TOKEN` cannot push cross-repo), tag-triggered release workflow (`fetch-depth: 0`, `contents: write`, `goreleaser-action@v7`, `version: "~> v2"`), attestations deferred (commented-out follow-up step), claudebox `COPY` snippet in `docs/` using the deterministic asset URL (`…/releases/download/{tag}/constitution_{version}_linux_amd64.tar.gz`).
**DoD:** `v0.1.0` released by CI; `brew install kentra-io/tap/constitution` and `go install …/cmd/constitution@v0.1.0` both yield a working, correctly-versioned binary; the linux asset URL resolves and the binary runs in a scratch container.

### M7 — Harness acceptance (feeds mvp-plan Phase 1)
On the greenfield testbed (`kafka-dq` shell): full adoption pass — `/constitution-init` interview → founding ADRs → append + supersede via conversation → planted-violation gate run. Capture friction as issues; write the companion-sync notes back to the harness docs (planning.md §7/§8 already point here).
**DoD:** the mvp-plan Phase-1 constitution-related DoD items are demonstrably satisfied by this primitive standalone (bootstrap, append/supersede + regen, violation surfaced, amendment requires consent).

Sequencing notes: M1→M2→M3 are strictly ordered on the domain core; M4 depends on M2; M6 can start any time after M0 (config is testable with snapshot releases) but the `v0.1.0` cut waits for M5.5 + the user-interactive founding re-seed (§2.12); M7 is last. Estimated effort concentration: M2 (byte-preservation + atomicity) and M5 (prompt design + live validation) are the two riskiest; everything else is routine Go.

---

## 8. Testing strategy (cross-cutting — high coverage is a stated v1 requirement)

The design is deliberately test-friendly: everything in Layer 1 is a deterministic function of files on disk, so **every behavior except the M5 live-agent checks is verifiable without an LLM**. Seven test classes, all wired as required CI jobs:

- **Unit** (table-driven): parser, patch editor, id allocation/renumber, config load/validation, active-set resolution, grouping/sorting, managed-block locator, manifest canonicalization. Every error path asserts the exact message contract (file/line/field) — errors are UX here, agents parse them.
- **Golden**: `regen` output byte-for-byte (`testdata/`, `-update` flag); also the managed-block writer, the rendered pointer blocks, and `deviation.json`/`guard --format json` against committed JSON-schema fixtures. The golden corpus grows with every bug: a regression fix is not done until it adds a fixture.
- **E2E** (testscript, black-box against the real binary): one scenario file per command plus full-lifecycle scripts (init → new → supersede → deprecate → renumber → guard), including failure scenarios (bad category, missing source, malformed body, unknown schemaVersion, dirty managed block). Run on the full OS matrix — Windows path/rename behavior is a first-class target, not an afterthought.
- **Property/differential**: (a) *projection idempotence* — `regen ∘ regen ≡ regen`; (b) *patch minimality* — for arbitrary valid ADRs, after `supersede` the byte-diff of the old file touches only the `status:`/`superseded-by:` lines (asserted mechanically, not by eyeball); (c) *parse↔render agreement* — the patched file re-parses to the same model with only status fields changed.
- **Fuzz** (native `go test -fuzz`): frontmatter split + status-line patch — the hand-rolled byte-level code is exactly what fuzzing exists to harden. Committed seed corpus; seed-only on every PR, scheduled 5-minute fuzz job weekly.
- **Crash-injection**: kill the process between each pair of writes in the multi-file sequences (M2 DoD) — proves the "log is truth, `regen` self-heals" recovery story on all three OSes.
- **Determinism**: repeated-run (×100) + cross-OS byte-equality of `regen` output in CI — guards against map-iteration and locale/collation regressions.
- **Live-agent** (M5 only): scripted acceptance sessions per the M5 DoD — consent checkpoint observed, planted violation cited by ADR-id, constitution demonstrably in context.

**Coverage gate:** `go test -coverprofile` enforced in CI — start at **85% overall on `internal/...`** and ratchet up (never down; the threshold is committed and a PR may only raise it). The core packages (`adr`, `patch`, `render`, `guard`) target ≥95% — they are small, pure, and have no excuse. Coverage is a floor, not the goal: the DoD test classes above (byte-preservation, crash-injection, determinism) are the actual quality bar, since 100% line coverage proves none of them.

## 9. Prerequisites on the owner's side (one-time setup)

Everything the build needs that only the org owner can do. Items 1–3 unblock M0; 4–5 are needed by M6.

1. **Create the repos**: `kentra-io/adr-sourced-constitution` (public) and `kentra-io/homebrew-tap` (public — Homebrew requires the `homebrew-` prefix and taps are public by convention). Wire the existing submodule's remote to the former.
2. **Create the agent's GitHub identity** — a **machine-user account**: **`kentra-gh-bot`** (created 2026-07-02), which mvp-plan §9 already plans for the harness ("single GitHub App/bot account"); this primitive is the first consumer. Machine accounts are explicitly permitted by GitHub ToS (an account operated by automation, owned by a human). Make it an **org member** of `kentra-io` (free/unlimited on the Free plan) with **write** on both repos (direct grant or via a `bots` team) — membership, not outside-collaborator status, is required because fine-grained PATs are documented as unusable for outside collaborators (classic-PAT-only gap in GitHub's docs). The heavier alternative — a **GitHub App** (commits as `<app>[bot]`, short-lived installation tokens) — is the better *Phase-2* answer when the engine runs unattended; for an interactive sandboxed agent the token-minting ceremony (JWT signing per hour) is friction without a security win, since a human supervises the session.
3. **Mint the agent's token**: a **fine-grained PAT owned by the machine account**, scoped to the two repos, permissions: `Contents: read/write`, `Pull requests: read/write`, and — easy to miss — **`Workflows: read/write`** (M0 pushes `.github/workflows/*`; pushes touching workflow files are rejected without it). Check org settings → third-party access → fine-grained PATs are allowed for `kentra-io`. Hand it to the sandbox as `GH_TOKEN`/`GITHUB_TOKEN` (see below).
4. **Release secret**: a second fine-grained PAT (same account) scoped to `homebrew-tap` only, `Contents: read/write`, stored as the `HOMEBREW_TAP_TOKEN` Actions secret on the main repo (GoReleaser cannot push cross-repo with the default `GITHUB_TOKEN`).
5. **Later (Phase 2, explicitly not now)**: branch protection / required `guard` check on `constitution/adr/**`, release attestations, Windows code-signing.

**Sandbox mechanics** (how the credential reaches the agent): the PAT lives in the macOS Keychain, surfaces on the host as **`KENTRA_BOT_GH_TOKEN`** (exported in `~/.zshrc` via `security find-generic-password`), and is mapped into the box by `.claudebox/config.yaml` (`env: {GITHUB_TOKEN: ${KENTRA_BOT_GH_TOKEN}, GH_TOKEN: ${KENTRA_BOT_GH_TOKEN}}` — the config file is git-tracked, so only the passthrough reference is committed, never the token). Git operations then work either via `gh auth setup-git` (if the `gh` CLI is installed in the image — recommended addition) or a one-line credential helper (`git config credential.helper '!f() { echo username=kentra-gh-bot; echo password=$GH_TOKEN; }; f'`) with HTTPS remotes. Set `git config user.name "kentra-gh-bot"` / `user.email` (the machine account's noreply address) inside the box so commits are attributed to the bot identity, with the human as PR reviewer/approver — which is exactly the consent topology §2.4 wants.

## 10. Risks

| Risk | Mitigation |
|---|---|
| Constitution outgrows ~200-line adherence guidance as ADRs accumulate | `regen` warning now; category-scoped/summary projection as a fast-follow (§12 of spec already anticipates rollups) |
| AGENTS.md pointer silently ignored by some tool | Gate is the correctness backstop (§2.1); governance-skill force-inline covers skill-capable tools; per-tool eval is spike §11.1 |
| Windows atomic-rename semantics are best-effort | `MoveFileEx` + crash-injection tests on windows CI (spike §11.2); worst case is a torn *projection*, which `regen` rebuilds |
| Consent gate bypassable by an agent writing files directly | Documented honestly (§2.4): permission-prompt checkpoint in v1; `guard` in CI catches out-of-band `adr/` edits; hard enforcement is Phase 2 by design |
| Concurrent-branch id collisions | `guard` id-uniqueness in CI + `renumber` escape hatch + documented branch-protection advice (§2.6) |
| Upstream shifts (Claude Code AGENTS.md support #6235 landing, GoReleaser/skill-format churn) | Versioned managed block makes pointer-strategy changes non-breaking; distribution config is pinned and small; re-verify at M0/M6 |

## 11. Remaining spikes (live experiments — scheduled, not blockers)

1. **Pointer-follow eval (M5):** the exact "read file X before planning" pattern has no published per-tool benchmark; validate with real sessions (Claude Code + ≥1 AGENTS.md-only tool). Also re-verify: Codex's rumored 32 KiB AGENTS.md truncation (secondary-source only), Gemini CLI's current default context file, Cursor global-skills support.
2. **Windows atomicity (M2):** prototype `MoveFileEx` vs best-effort `os.Rename` under crash injection on a windows-latest runner.
3. **Carried forward (out of v1 with the adapters):** Spec-Kit `after_constitution` overwrite (§13.2), OpenSpec native-ADR collision / upstream MADR-projection convention proposal (§13.4).

## 12. Deferred (unchanged from spec §12 + P1)

Framework adapters (Spec-Kit first when picked back up) · drift sweep · brownfield extraction · log rollup/snapshot · synthesized prose · ubiquitous-language integration · code-time check · `advisory`/scoped consent modes · `--format github` annotations · release attestations · Claude-Code-plugin packaging of the skills (open question from research; revisit with adapter work).

## 13. Component → milestone map

| Spec §11 component | Milestone |
|---|---|
| `constitution` CLI (7 verbs incl. `renumber`) | M0–M4 |
| ADR record + schema, immutability guard, projection | M1–M3 |
| `constitution-init` interview, ADR draft, plan gate, governance SKILL.md | M5 (packaging M4) |
| Default folder + managed pointer | M4 |
| Spec-tracking + consent seams | M2/M4 (config-level) |
| Distribution (GoReleaser/tap/claudebox) | M6 |
| Harness Phase-1 acceptance | M7 |

## 14. Provenance

- v1 scoping decisions P1–P4: user session, 2026-07-02.
- Research: 6 parallel research agents + completeness critique (2026-07-02), all library/framework claims verified against live primary sources — MADR v4 templates (`adr/madr`), Anthropic memory/skills docs + changelog, agents.md/AAIF, GoReleaser docs + deprecations, formulae.brew.sh API, urfave/cli v3, yaml org migration threads, goccy #285, google/renameio docs, go.dev release policy, rogpeppe/go-internal, adr-tools #102, log4brains id ADR, OpenSpec/Spec-Kit scaffolder docs, SARIF 2.1.0, Spec-Kit `analyze` template.
- Notable negative results honored: `brews` removed from GoReleaser; `gopkg.in/yaml.v3` archived; `google/renameio` unusable on Windows; no existing tool renders an active-set projection (novelty reconfirmed at tool level, not just framework level).
