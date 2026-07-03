<!--
  GENERATED FILE -- projection of the ADR log in constitution/adr/.
  Do not hand-edit; changes will be overwritten by the next "constitution
  regen". To change a rule, add, supersede, or deprecate an ADR instead.
-->

# Constitution

## architecture

### Integrate via a hybrid per-tool pointer strategy

Integrate via a hybrid, per-tool pointer strategy. CLAUDE.md receives a true inline `@constitution/constitution.md` import; AGENTS.md receives a short textual pointer naming the file as the governing constitution that outranks inferred conventions. The governance skill additionally force-inlines the constitution independent of pointer compliance. `regen` warns (does not block) when constitution.md exceeds ~200 lines.

ADR-0001 · 2026-07-03

### Write the managed pointer block in a versioned marker-delimited format

Write the managed pointer as a versioned, marker-delimited block using the doctoc/terraform-docs pattern: a `BEGIN adr-sourced-constitution v1 … / END` HTML-comment pair. Locate by the exact marker pair and rewrite only the interior; append a new block at EOF if absent. Store a hash of the last-written interior in `constitution/.state`; if the interior drifted, show a diff and require confirm or `--force`. The `v1` token is how a future strategy change migrates non-breakingly.

ADR-0002 · 2026-07-03

### Allocate sequential ids with a renumber escape hatch

Allocate sequential zero-padded `ADR-NNNN` ids by scanning for the highest existing id. Treat collisions as a CI-time problem: `guard` includes an id/filename-uniqueness check, and adopters enable up-to-date-branch or merge-queue protection. Provide `constitution adr renumber <old> <new>` as the escape hatch — a pure rename plus frontmatter id edit, refused if any ADR references the old id.

ADR-0006 · 2026-07-03

### Type the source-ref contract in config

Type the source-ref contract in `constitution.yml` under `sourceTracking`: `type` is one of `none | generic | github-issue | jira`, with an optional `pattern` regex (defaults per type). When `type` is `none`, no ADR may carry a `source`; otherwise `source` is required and shape-checked. Founding ADRs use the reserved `bootstrap` source. No live tracker integration in v1.

ADR-0008 · 2026-07-03

### Fix the deviation.json contract as SARIF-shaped and CLI-validated

Fix the deviation.json contract as a SARIF-shaped JSON (fields: generatedAt, constitutionHash, plan, deviations[], summary) using the CRITICAL/HIGH/MEDIUM/LOW severity vocabulary; every deviation cites a required `adrId`. Default output `./deviation.json`. Validation is CLI-owned via the hidden `constitution deviation validate <path>` verb (exit 0 valid / 1 invalid / 2 could-not-run): it schema-checks the report, confirms every adrId cites an active ADR in the log and the summary counts tally, and advises (HIGH, non-fatal) when constitutionHash no longer matches constitution.md.

ADR-0009 · 2026-07-03

### Keep project config in a versioned root constitution.yml

Keep project config in a versioned `constitution.yml` at the repo root, a sibling of `constitution/`. Parse it with a plain YAML unmarshal into a `schemaVersion: 1` struct; an unrecognized schemaVersion is refused with a clear message (no migration machinery in v1). No Viper. Fields cover agent-instruction targets, consent policy, sourceTracking, and the category vocabulary.

ADR-0010 · 2026-07-03

### Pin the byte-fidelity stack for deterministic projection

Pin the byte-fidelity stack. Use `go.yaml.in/yaml/v3` everywhere (the maintained successor to the archived yaml.v3), but only to validate — never to rewrite an accepted ADR. Change an ADR's status with a line-targeted textual patch (find `^status:`, replace the value; insert `superseded-by:` after it) so untouched bytes are preserved exactly. Write every file atomically via a same-dir temp file plus rename (MoveFileEx on Windows). The log is truth: a crash mid-sequence always leaves a self-consistent log that regen re-derives.

ADR-0011 · 2026-07-03

## process

### Mediate every ADR write through the constitution CLI

Mediate every ADR write through the constitution CLI. `constitution adr new --title … --category … --body-file <path|->` supplies the MADR body only; the CLI validates the mandatory sections, allocates the id, composes frontmatter, writes atomically, and auto-runs regen. `supersede`/`deprecate` take a target id. Agents never write into `adr/` directly; a skill drafts to a temp file and invokes the CLI only on human acceptance, so a rejected draft never touches the log.

ADR-0003 · 2026-07-03

### Gate mutating commands behind a strict-or-off consent policy

Gate mutating commands behind a `strict | off` consent policy stored in config. Under `strict` (the default) every `adr new`/`supersede`/`deprecate` requires an interactive TTY confirm or `--approve` for scripted use; the shipped skills deliberately do not pre-grant these commands in `allowed-tools`, so the human approves each write at the harness permission prompt. `off` removes the CLI-level gate. Advisory/scoped/batched modes are deferred. Hard enforcement beyond the permission boundary is Phase 2.

ADR-0004 · 2026-07-03

### Govern the category vocabulary in constitution.yml

Govern the category vocabulary in `constitution.yml`. `adr new` hard-errors on an unknown category; introducing one requires `--new-category`, which appends it to config and still produces an ordinary ADR (no meta-record type). `init` proposes the starter list (architecture, code-style, process, testing, security, data) as a suggestion only. The projection groups categories in config order.

ADR-0005 · 2026-07-03

### Guard the log with git and advisory-manifest modes

Guard the log with git and advisory-manifest modes. Git mode shells out to the system git and does a structured frontmatter+body comparison allowing only `status:`/`superseded-by:` to differ; CI uses `git merge-base` computed locally. A `.manifest.sha256` records each ADR's frozen-content hash, rewritten by every mutating command and always cross-checked — advisory, with no tamper-evidence claim against a malicious committer. Exit codes: 0 clean, 1 violations, 2 could-not-run; `--format json` emits a typed violation payload. v1 ships one documented, advisory GitHub Actions example.

ADR-0007 · 2026-07-03
