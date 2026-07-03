---
id: ADR-0001
title: Integrate via a hybrid per-tool pointer strategy
category: architecture
date: 2026-07-03
status: deprecated
---

## Context and Problem Statement

Agents load project instructions from tool-specific files (CLAUDE.md, AGENTS.md, and others). We must make the constitution reliably present in an agent's context at planning time, across tools with different import capabilities, without depending on any single tool's behavior (implementation-plan.md §2.1, §5; spec §13.1).

## Decision Drivers

- Claude Code expands `@path` imports into context at launch; it does not yet read AGENTS.md.
- AGENTS.md is a cross-tool standard but has no import primitive; short, concrete, single-file pointers are the regime where pointers are actually followed.
- Pointer-following is load-bearing for UX quality, not for correctness — the plan-gate is the correctness backstop.

## Considered Options

- One universal pointer file for all tools.
- A true inline import everywhere.
- A hybrid strategy decided per-tool by the CLI.

## Decision Outcome

Integrate via a hybrid, per-tool pointer strategy. CLAUDE.md receives a true inline `@constitution/constitution.md` import; AGENTS.md receives a short textual pointer naming the file as the governing constitution that outranks inferred conventions. The governance skill additionally force-inlines the constitution independent of pointer compliance. `regen` warns (does not block) when constitution.md exceeds ~200 lines.
