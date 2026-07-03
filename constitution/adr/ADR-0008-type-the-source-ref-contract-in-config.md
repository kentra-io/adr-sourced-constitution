---
id: ADR-0008
title: Type the source-ref contract in config
category: architecture
date: 2026-07-03
status: deprecated
---

## Context and Problem Statement

An ADR may reference the issue or ticket that motivated it, but tracker formats differ and v1 does no live tracker integration. We must define how a `source` ref is typed and validated (implementation-plan.md §2.8; spec §13.3).

## Decision Drivers

- The `source` field must be validated for presence and shape, not resolved live.
- Different projects use different trackers (GitHub issues, Jira) or none.
- Founding ADRs need a reserved source that satisfies a non-none policy.

## Considered Options

- A free-form string with no validation.
- Live tracker lookups.
- A typed config contract with per-type default patterns and a reserved bootstrap value.

## Decision Outcome

Type the source-ref contract in `constitution.yml` under `sourceTracking`: `type` is one of `none | generic | github-issue | jira`, with an optional `pattern` regex (defaults per type). When `type` is `none`, no ADR may carry a `source`; otherwise `source` is required and shape-checked. Founding ADRs use the reserved `bootstrap` source. No live tracker integration in v1.
