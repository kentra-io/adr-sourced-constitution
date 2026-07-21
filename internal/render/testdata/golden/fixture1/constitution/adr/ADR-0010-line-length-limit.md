---
id: ADR-0010
title: Limit source lines to 100 characters
date: 2026-06-21
status: accepted
source: FS-0010
---

## Context and Problem Statement

Very long lines are hard to review side-by-side and in terminal diffs.

## Considered Options

- No line-length limit
- A soft 100-character limit, enforced by linter warning

## Decision Outcome

Source lines should not exceed 100 characters; the linter warns, it does
not hard-fail the build.
