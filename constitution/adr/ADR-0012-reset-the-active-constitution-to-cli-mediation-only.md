---
id: ADR-0012
title: Reset the active constitution to CLI-mediation only
category: process
date: 2026-07-03
status: deprecated
---

## Context and Problem Statement

The constitution had accumulated eleven founding ADRs covering the full v1 CLI
design — pointer strategy, marker format, id allocation, source-ref contract,
deviation report, config schema, byte-fidelity stack, consent policy, category
vocabulary, and guard modes. We have decided to reset the active constitution
down to a single governing rule and retire the rest, so the log carries one
minimal, load-bearing rule rather than the full founding set. This record
documents why that reset happened and which rule is deliberately retained.

## Decision Drivers

- The reset must be auditable: the log must explain why ten rules were retired,
  not silently drop them.
- The append-only invariant must be preserved — rules are retired by
  deprecation, never deleted.
- Exactly one rule is worth keeping active: that every ADR write is mediated
  through the constitution CLI, which is what protects the log's integrity.

## Considered Options

- Deprecate the other ten ADRs and record the reset decision as a new ADR.
- Deprecate the other ten ADRs silently, leaving no record of the reset.

## Decision Outcome

Reset the active constitution to CLI-mediation only. ADR-0003 ("Mediate every
ADR write through the constitution CLI") MUST remain the sole retained founding
rule; ADR-0001, 0002, 0004, 0005, 0006, 0007, 0008, 0009, 0010, and 0011 MUST
be deprecated, retiring them from the projection while preserving them in the
append-only log. This record exists so the reset is explainable from the log:
the CLI-mediation rule is retained because it is the invariant that keeps every
future change to the log governed.
