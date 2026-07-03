---
id: ADR-0010
title: Keep project config in a versioned root constitution.yml
category: architecture
date: 2026-07-03
status: accepted
---

## Context and Problem Statement

The project needs configuration (integration targets, consent policy, source tracking, category vocabulary) that is neither part of the append-only log nor of the generated projection. We must decide where it lives and how schema evolution is handled (implementation-plan.md §2.10).

## Decision Drivers

- Config is a third thing, distinct from log and projection.
- Schema evolution must fail loudly rather than silently mis-parse.
- Multi-source config merging is unneeded weight.

## Considered Options

- Embed config inside the constitution/ tree.
- Use a multi-source config loader (e.g. Viper).
- A single versioned YAML file at repo root, plain-unmarshaled.

## Decision Outcome

Keep project config in a versioned `constitution.yml` at the repo root, a sibling of `constitution/`. Parse it with a plain YAML unmarshal into a `schemaVersion: 1` struct; an unrecognized schemaVersion is refused with a clear message (no migration machinery in v1). No Viper. Fields cover agent-instruction targets, consent policy, sourceTracking, and the category vocabulary.
