---
id: ADR-0003
title: Require PR review before merging an ADR
category: process
date: 2026-06-03
status: accepted
source: FS-0003
---

## Context and Problem Statement

Append-by-construction (spec §5.1) still needs a human checkpoint before
an ADR lands on the default branch.

## Considered Options

- Trust the author, no review
- Require at least one approving PR review

## Decision Outcome

Every ADR must be merged via a pull request with at least one approval.
