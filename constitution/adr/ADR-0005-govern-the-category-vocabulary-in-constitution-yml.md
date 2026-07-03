---
id: ADR-0005
title: Govern the category vocabulary in constitution.yml
category: process
date: 2026-07-03
status: deprecated
---

## Context and Problem Statement

ADRs are grouped by category in the projection, so the category vocabulary is part of the contract. We must decide where the vocabulary lives and how a new category is introduced without letting typos silently fragment the log (implementation-plan.md §2.5; spec §13.6).

## Decision Drivers

- An unknown category is almost always a typo, not an intent.
- Introducing a category should be deliberate but should not require a separate meta-record type.
- Grouping order in the projection follows config order and must be stable.

## Considered Options

- Free-form categories, no validation.
- A separate meta-record to register a category.
- A fixed vocabulary in config; unknown category hard-errors unless explicitly introduced.

## Decision Outcome

Govern the category vocabulary in `constitution.yml`. `adr new` hard-errors on an unknown category; introducing one requires `--new-category`, which appends it to config and still produces an ordinary ADR (no meta-record type). `init` proposes the starter list (architecture, code-style, process, testing, security, data) as a suggestion only. The projection groups categories in config order.
