# Manifest canonicalization — design note

*Finalizes the canonicalization rule that `internal/manifest` implements and
`constitution guard` cross-checks (implementation-plan.md §2.7, spec §5.4).
This is the authoritative description; the package doc in
`internal/manifest/manifest.go` points here.*

## What the manifest is

`constitution/adr/.manifest.sha256` is a `sha256sum`-style file — one
`<hex>  <filename>` line per ADR, sorted by filename:

```
8ef859cdc708fe822eaf7f81c951d7e6fcd085a57334d81db8b1fd41f96f19ba  ADR-0001-first-rule.md
```

Each hash is the SHA-256 of an ADR's **canonical frozen content**. Every
mutating command (`adr new`, `supersede`, `deprecate`, `renumber`) rewrites
the whole file as its final step, via `regen`. `guard` recomputes each hash
from disk and compares against the recorded value: a mismatch means the file
changed without going through the CLI.

## Canonicalization rule

The hash is taken over a deterministic byte encoding produced from the
**parsed** ADR model — never the raw file bytes. Working from the model, not
the bytes, is what makes the hash line-ending- and BOM-independent: a
CRLF-authored ADR and its LF twin parse to the same model and therefore hash
identically (a tested invariant). The encoding is internal — it is never
written to disk — so it optimizes purely for being unambiguous and stable
across runs and platforms.

### 1. Included fields (frozen only)

Included, in this fixed order:

| Part | Source |
|------|--------|
| `id` | frontmatter |
| `title` | frontmatter |
| `category` | frontmatter |
| `date` | frontmatter (date **created**, frozen forever — see errata §1.2) |
| `source` | frontmatter (empty when absent) |
| `supersedes` | frontmatter (empty when absent) |
| body sections | each `##` heading + its content, in file order |

### 2. Excluded fields (mutable)

`status` and `superseded-by` are **deliberately excluded**. They are the only
two fields a legal status transition may change (spec §5.2:
`accepted -> superseded | deprecated`). Excluding them is what makes the
manifest cooperate with the immutability model:

- a **legal** supersede/deprecate does **not** alter the target ADR's hash,
  so the manifest changes only by *gaining* the new ADR's line — no false
  positive;
- any **illegal** edit to frozen content (a reworded Decision Outcome, a
  changed category or title) **does** change the hash and is detectable, even
  with no git history to diff against.

A consequence worth stating plainly: because status is excluded, the manifest
alone cannot catch an *illegal status transition* (e.g. a resurrection). That
class is caught by git-mode legality instead (`internal/guard/legality.go`).
The two checks are complementary, not redundant — see the `guard_merge_base`
testscript, where an illegal committed status edit is invisible to the
manifest and caught only by the git-mode structured comparison.

### 3. Section-count pin

Before the body sections, the encoding writes `sections <N>\n` where `N` is
the number of sections. Pinning the count stops a section from being silently
added or removed without changing the structure the hash commits to.

### 4. Length-prefixed (netstring) framing — injection-proofing

Every value is written as `name <len>:<bytes>\n` (a netstring-style length
prefix), for both frontmatter fields and each section heading/content:

```
id 8:ADR-0001
title 5:First rule      <- (len is of the value, illustrative)
...
sections 3
heading 30:Context and Problem Statement
content 21:We need a rule here.
```

The length prefix pins each value's exact extent, which makes the encoding
**injection-proof**: a value containing embedded newlines, or text that
mimics the framing (YAML block scalars and quoted scalars both allow this),
cannot shift a field boundary. A naive `name:value\n` scheme is forgeable —
title `"T\ncategory:X"` with category `"c"` would encode to the same bytes as
title `"T"` with category `"X\ncategory:c"`, colliding two distinct ADRs onto
one hash. The length prefix closes that. (Asserted by
`TestCanonicalizeInjection` in `internal/manifest`.)

## Honest scope: advisory in v1

The manifest is **advisory**. It gives no tamper-evidence guarantee against a
malicious committer: nothing stops someone editing an ADR **and** rewriting
its manifest line in the **same commit**, which the cross-check cannot
distinguish from a legitimate CLI-produced change. What it *does* reliably
catch:

- an out-of-band edit that forgot to run the CLI (the common, honest case);
- an edit whose containing commit was later amended or rebased away, leaving
  git-mode nothing to diff — here the manifest is the *only* check that fires
  (the `guard_history_rewrite` testscript).

Hard tamper-evidence — branch protection, a required `guard` check on
`constitution/adr/**`, signed commits — is Phase 2, deliberately out of v1
scope (plan §2.7, §9.5). The `docs/ci-guard-example.yml` job is likewise
advisory and explicitly non-required.
