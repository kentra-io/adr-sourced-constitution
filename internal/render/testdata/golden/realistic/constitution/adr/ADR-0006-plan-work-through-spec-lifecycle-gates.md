---
id: ADR-0006
title: Plan work through spec-lifecycle gates
date: 2026-06-06
status: accepted
---

## Context and Problem Statement

Work reaches code through staged, gated planning artefacts rather than
ad-hoc implementation, so that intent is reviewable before it is built and
reviewers do not have to reconstruct rationale from a diff after the
fact.

## Considered Options

- Ad-hoc implementation, with review happening only at the pull-request
  stage
- Staged, gated planning through spec-lifecycle, with review happening
  before implementation starts

## Decision Outcome

Take all non-trivial work through spec-lifecycle. Read the relevant
`openspec/changes/<change>/` artefacts before touching related code, and
run `lifecycle status` to see gate state. Approve gates only via
`lifecycle approve` — never by hand-editing `approval-state.json`.

## Rules

### process

#### spec-lifecycle-gates

Take all non-trivial work through spec-lifecycle. Read the relevant
`openspec/changes/<change>/` artefacts before touching related code, and
run `lifecycle status` to see gate state. Approve gates only via
`lifecycle approve` — never by hand-editing `approval-state.json`.
