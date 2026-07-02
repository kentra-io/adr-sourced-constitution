---
id: ADR-0001
title: Model the constitution as an event-sourced ADR log
category: architecture
date: 2026-06-01
status: superseded
source: FS-0001
superseded-by: ADR-0005
---

## Context and Problem Statement

The constitution needs a source of truth that supports supersede/deprecate
semantics and per-rule citations.

## Considered Options

- Hand-edit constitution.md directly
- Event-sourced ADR log projected to constitution.md

## Decision Outcome

Model the constitution as a projection of an append-only ADR log.

## Consequences

Every governance change becomes an ADR; the projection is regenerated,
never hand-edited.
