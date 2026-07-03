---
id: ADR-0007
title: Guard the log with git and advisory-manifest modes
category: process
date: 2026-07-03
status: accepted
---

## Context and Problem Statement

The append-only invariant needs a detector: something that flags out-of-band mutation of an accepted ADR (a body edit, a frozen-field change, a deletion or rename), both locally and in CI, including environments without git (implementation-plan.md §2.7; spec §5.3, §5.4).

## Decision Drivers

- A structured record comparison, not a hand-parsed diff, is what correctly allows only legal status transitions.
- CI must compute the merge base locally and never trust a PR-supplied base SHA.
- A manifest hash is the only check that works without git, but it is advisory (a committer can edit both files in one commit).

## Considered Options

- Git-only checking.
- Manifest-only checking.
- Both: git mode by default, manifest cross-check always, degrade to manifest-only without git.

## Decision Outcome

Guard the log with git and advisory-manifest modes. Git mode shells out to the system git and does a structured frontmatter+body comparison allowing only `status:`/`superseded-by:` to differ; CI uses `git merge-base` computed locally. A `.manifest.sha256` records each ADR's frozen-content hash, rewritten by every mutating command and always cross-checked — advisory, with no tamper-evidence claim against a malicious committer. Exit codes: 0 clean, 1 violations, 2 could-not-run; `--format json` emits a typed violation payload. v1 ships one documented, advisory GitHub Actions example.
