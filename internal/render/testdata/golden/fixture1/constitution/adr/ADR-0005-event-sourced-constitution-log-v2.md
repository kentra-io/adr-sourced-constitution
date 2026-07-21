---
id: ADR-0005
title: Model the constitution as an event-sourced ADR log, revised
date: 2026-06-10
status: superseded
source: FS-0005
supersedes: ADR-0001
superseded-by: ADR-0009
---

## Context and Problem Statement

ADR-0001 didn't pin the render mechanism (agent-synthesized vs.
deterministic), which the gate needs to be stable.

## Considered Options

- Agent-synthesized prose render
- Deterministic text/template render

## Decision Outcome

Render constitution.md deterministically from the active ADR set.

## Consequences

Rules out agent-synthesized constitution prose (spec §12).
