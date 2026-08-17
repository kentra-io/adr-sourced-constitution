---
id: ADR-0007
title: Defer the JVM framework choice
date: 2026-06-07
status: accepted
---

## Context and Problem Statement

Both applications will use a JVM framework, but the choice between
Micronaut and Quarkus has not been evaluated against this project's
needs. Deciding now, before either has been tried against a real
hexagonal boundary, risks locking in a framework whose defaults fight the
architecture.

## Considered Options

- Pick a framework now, based on general reputation
- Defer the choice to stage-2 research, once both have been evaluated
  against this project's hexagonal boundaries

## Decision Outcome

Defer the JVM framework choice to stage-2 research; neither Micronaut nor
Quarkus has been evaluated against this project's needs. Recorded here so
the deferral is explicit and so no downstream work assumes an answer. The
binding constraint that survives whichever framework wins is
`hexagonal-per-skill` (ADR-0004): the framework stays confined to the
composition root.

## Consequences

This is a record-only decision and establishes no standing rule; it has
no `## Rules` section and never projects into constitution.md.
