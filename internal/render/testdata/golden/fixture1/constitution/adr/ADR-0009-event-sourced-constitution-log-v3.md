---
id: ADR-0009
title: Model the constitution as an event-sourced ADR log, final form
date: 2026-06-20
status: accepted
source: FS-0009
supersedes: ADR-0005
---

## Context and Problem Statement

ADR-0005 pinned deterministic rendering but left the record schema
(MADR compliance, immutability model) unresolved.

## Considered Options

- Freeform per-project ADR schema
- Minimal-MADR-compliant schema with a mutable `status` field

## Decision Outcome

Every ADR is a minimal-MADR-compliant record with a mutable `status`
field (spec §4.1, §5); constitution.md is the deterministic projection
of the ADRs with `status: accepted`.

## Consequences

Supersedes ADR-0005; closes out the architecture question this ADR
chain worked through.

## Rules

### architecture

#### deterministic-projection

Model the constitution as a deterministic projection of the accepted ADR
set; never hand-author constitution.md.
