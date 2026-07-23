---
id: ADR-0013
title: "Founding phase: draft until sealed"
date: 2026-07-23
status: accepted
---

## Context and Problem Statement

v0.1 made the ADR log append-only starting from the very first ADR: every
constitution, from its founding record onward, was already under full
sealed-phase discipline. Two live founding attempts on this tooling
(kafka-dq, 2026-07) each ended in a full constitution wipe, because the
early, still-forming ADRs of a founding session could not be cheaply
corrected once written — the only path to fix a wrong early decision was to
discard the whole log and start over. We need a way for founding decisions
to be provisional without weakening the guarantees a sealed constitution
provides once founding is done.

## Decision Drivers

- Founding decisions are provisional by nature: a project's first ADRs are
  drafted before its technology bets are validated, and getting one wrong
  should not be catastrophic.
- Append-only theater during drafting — writing an ADR knowing it may need
  correcting, with no cheap way to correct it — is what forced the resets.
- Whatever founding-phase flexibility we add must not weaken the append-only,
  tamper-evident guarantees a project relies on once its constitution is
  actually sealed.

## Considered Options

- Keep append-only from birth (v0.1 status quo): simplest, but reset is the
  only recourse for an early mistake.
- Add an authorized re-baseline escape hatch: lets a human explicitly rewrite
  history, but blurs the append-only guarantee and needs its own audit trail.
- Add an explicit, mutable draft phase that ends in a one-way seal: founding
  work is cheap to correct, and the append-only guarantee begins exactly when
  the human says founding is done.

## Decision Outcome

Chosen option: an explicit mutable draft phase ending in a one-way seal.
`constitution.yml` requires a `phase: draft | sealed` field, with no
absent-field defaulting — pre-v0.2 configs must add the line explicitly.
In `draft` phase the log is a fully mutable working set: consent-gated
`adr edit` and `adr rm` are available, and no manifest baseline exists yet.
`constitution seal` is the one-way transition: it writes the manifest
baseline and flips the log append-only forever from that point on. Sealed-
phase semantics are exactly the v0.1 semantics, unchanged.

## Consequences

Founding mistakes are cheap to fix until a project seals; sealing becomes an
explicit, deliberate human act rather than an implicit property of having
written a first ADR. Every pre-v0.2 `constitution.yml` needs the one-line
`phase:` field added by hand before it validates again; there is no
compatibility tooling to add it automatically — in-house logs were
hand-migrated.
