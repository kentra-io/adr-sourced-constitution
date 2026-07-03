---
id: ADR-0003
title: Mediate every ADR write through the constitution CLI
category: process
date: 2026-07-03
status: accepted
---

## Context and Problem Statement

An ADR body is multi-paragraph MADR prose, but the log is append-only and its integrity depends on records being well-formed and never silently mutated. We must define how a Layer-2 agent skill gets a record into `constitution/adr/` (implementation-plan.md §2.3; spec §7.1).

## Decision Drivers

- "The accept IS the write" — the write is the governance event.
- The CLI must own id allocation, frontmatter composition, shape validation, atomic write, and re-projection.
- Rejected drafts must never land in the log (append-by-construction).

## Considered Options

- Agent writes the Markdown file directly into `adr/`.
- Agent calls a CLI verb that takes the body and composes the record.

## Decision Outcome

Mediate every ADR write through the constitution CLI. `constitution adr new --title … --category … --body-file <path|->` supplies the MADR body only; the CLI validates the mandatory sections, allocates the id, composes frontmatter, writes atomically, and auto-runs regen. `supersede`/`deprecate` take a target id. Agents never write into `adr/` directly; a skill drafts to a temp file and invokes the CLI only on human acceptance, so a rejected draft never touches the log.
