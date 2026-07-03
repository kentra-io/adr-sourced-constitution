---
id: ADR-0002
title: Write the managed pointer block in a versioned marker-delimited format
category: architecture
date: 2026-07-03
status: deprecated
---

## Context and Problem Statement

The pointer/import text the CLI injects into a human-owned instruction file must be updatable in place, re-findable across CLI versions, and safe to migrate when the integration strategy changes, without clobbering the human's surrounding content (implementation-plan.md §2.2).

## Decision Drivers

- The instruction file is the user's, not part of the ADR log — edits to it are a soft concern, not a domain violation.
- A future pointer→inline strategy change must not break existing managed blocks.
- Marker text must stay byte-stable across CLI versions (the ansible-blockinfile failure mode).

## Considered Options

- Rewrite the whole target file each run.
- Append-only, never update.
- HTML-comment marker pair delimiting a versioned, in-place-rewritten block.

## Decision Outcome

Write the managed pointer as a versioned, marker-delimited block using the doctoc/terraform-docs pattern: a `BEGIN adr-sourced-constitution v1 … / END` HTML-comment pair. Locate by the exact marker pair and rewrite only the interior; append a new block at EOF if absent. Store a hash of the last-written interior in `constitution/.state`; if the interior drifted, show a diff and require confirm or `--force`. The `v1` token is how a future strategy change migrates non-breakingly.
