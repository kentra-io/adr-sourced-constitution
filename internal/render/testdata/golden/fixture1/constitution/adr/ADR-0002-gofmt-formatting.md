---
id: ADR-0002
title: Format all Go code with gofmt
category: code-style
date: 2026-06-02
status: accepted
source: FS-0002
---

## Context and Problem Statement

Inconsistent formatting produces noisy diffs and slows review.

## Considered Options

- No enforced formatter
- gofmt, enforced in CI

## Decision Outcome

All Go code must be formatted with gofmt; CI rejects unformatted diffs.

## Rule

Format all Go code with gofmt; unformatted diffs must not merge.
