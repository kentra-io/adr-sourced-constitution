---
id: ADR-0005
title: Open source under a permissive, redistribution-safe license
date: 2026-06-05
status: accepted
---

## Context and Problem Statement

An open-source promise is only real if every dependency permits
redistribution, so licence compatibility is a constraint on day-to-day
dependency choices, not just a one-time licence file at the repository
root.

## Considered Options

- A restrictive or unspecified project licence, with dependency licences
  reviewed only informally
- A permissive project licence, with every dependency's licence checked
  for redistribution compatibility before it is added

## Decision Outcome

Release this project under a permissive open-source licence (MIT or
Apache-2.0), and require every dependency to be compatible with
permissive redistribution.

## Rules

### process

#### permissive-license

Release this project under a permissive open-source licence (MIT or
Apache-2.0).

### tooling

#### license-compatible-dependencies

Every dependency must be compatible with permissive redistribution. Do
not introduce copyleft (GPL/AGPL/LGPL) or proprietary dependencies.
