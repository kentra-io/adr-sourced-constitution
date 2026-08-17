---
id: ADR-0009
title: Defer library packaging decisions
date: 2026-06-09
status: accepted
---

## Context and Problem Statement

Whether the engine library (`library-first-engine`, ADR-0002) is a
module inside this repository's build or a separately published artefact
is undecided, and committing to either now would constrain the build
tooling before the library's consumers are known.

## Considered Options

- Publish the engine as a separate artefact immediately
- Keep it as an in-repo module for now and decide packaging later

## Decision Outcome

Keep the engine library as an in-repo module for now; the packaging
question is deferred. `library-first-engine` constrains the dependency
direction and independent consumability, not the packaging mechanism.

## Consequences

Record-only; establishes no standing constraint.
