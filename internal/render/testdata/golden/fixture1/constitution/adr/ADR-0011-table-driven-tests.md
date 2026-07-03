---
id: ADR-0011
title: Prefer table-driven tests
category: testing
date: 2026-06-22
status: accepted
source: FS-0011
---

## Context and Problem Statement

Repeated near-identical test functions are harder to scan for coverage
gaps than a single table of cases.

## Considered Options

- One test function per case
- Table-driven tests with `t.Run` subtests

## Decision Outcome

Prefer table-driven tests with named `t.Run` subtests for anything with
more than two similar cases.

## Rule

Prefer table-driven tests with named subtests beyond two similar cases.
