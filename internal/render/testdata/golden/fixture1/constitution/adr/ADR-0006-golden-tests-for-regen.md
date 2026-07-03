---
id: ADR-0006
title: Require golden tests for the regen projection
category: testing
date: 2026-06-11
status: accepted
source: FS-0006
---

## Context and Problem Statement

`regen`'s output must be byte-identical across runs and operating
systems; that's hard to eyeball-review.

## Considered Options

- Manual review of rendered output on each change
- Committed golden fixtures with byte-exact comparison

## Decision Outcome

Every change to the projection ships a golden fixture asserting the
exact rendered bytes.

## Rule

Every projection change ships a byte-exact golden fixture.
