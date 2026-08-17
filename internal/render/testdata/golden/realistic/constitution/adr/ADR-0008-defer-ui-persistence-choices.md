---
id: ADR-0008
title: Defer UI persistence choices
date: 2026-06-08
status: accepted
---

## Context and Problem Statement

At this stage the UI application persists violations to container-local
storage. The datastore choice, durability guarantees, and retention
behaviour have not been decided, and deciding them now would be
speculative — there is not yet enough production usage to know what
durability actually needs to look like.

## Considered Options

- Commit to a specific external datastore now
- Keep container-local storage for now and defer the datastore decision

## Decision Outcome

Keep container-local storage for now. The datastore choice, durability
guarantees, and retention behaviour are deliberately unsettled and will
be decided later, once real usage informs the durability requirements.

## Consequences

Record-only; establishes no standing constraint. Persistence work should
not assume any particular datastore until this is revisited.
