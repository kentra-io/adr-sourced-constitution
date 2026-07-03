---
id: ADR-0006
title: Allocate sequential ids with a renumber escape hatch
category: architecture
date: 2026-07-03
status: accepted
---

## Context and Problem Statement

ADRs need stable, human-readable, monotonic identifiers, but concurrent branches can independently allocate the same next id, and a collision must be resolvable without rewriting history (implementation-plan.md §2.6; spec §4.1, §4.3).

## Decision Drivers

- Readability: `ADR-NNNN` beats date-slug ids.
- Optimistic allocation (highest id + 1) is simple and matches adr-tools.
- A colliding, not-yet-merged ADR is referenced by nothing, so a rename is safe.

## Considered Options

- Date-based slugs to avoid collisions.
- A lock file for id allocation.
- Sequential zero-padded ids, collisions caught at CI merge time, plus a renumber escape hatch.

## Decision Outcome

Allocate sequential zero-padded `ADR-NNNN` ids by scanning for the highest existing id. Treat collisions as a CI-time problem: `guard` includes an id/filename-uniqueness check, and adopters enable up-to-date-branch or merge-queue protection. Provide `constitution adr renumber <old> <new>` as the escape hatch — a pure rename plus frontmatter id edit, refused if any ADR references the old id.
