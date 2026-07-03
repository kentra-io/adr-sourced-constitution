---
id: ADR-0009
title: Fix the deviation.json contract as SARIF-shaped and CLI-validated
category: architecture
date: 2026-07-03
status: accepted
---

## Context and Problem Statement

The plan-gate produces a machine-readable report of where a plan conflicts with the constitution. Its schema must be stable, citable by ADR id, and validated by something authoritative rather than by prose an agent has to reproduce (implementation-plan.md §2.9; spec §8b).

## Decision Drivers

- The ADR-id citation is the governance primitive — every deviation must cite one.
- SARIF-shaped output keeps a future GitHub code-scanning upload a thin mapping; Spec-Kit severity vocabulary keeps the future adapter translation-free.
- The CLI, not the skill, must own schema and citation validation.

## Considered Options

- Free-form prose report.
- Raw SARIF.
- A purpose-built, SARIF-shaped JSON with a CLI-owned validator.

## Decision Outcome

Fix the deviation.json contract as a SARIF-shaped JSON (fields: generatedAt, constitutionHash, plan, deviations[], summary) using the CRITICAL/HIGH/MEDIUM/LOW severity vocabulary; every deviation cites a required `adrId`. Default output `./deviation.json`. Validation is CLI-owned via the hidden `constitution deviation validate <path>` verb (exit 0 valid / 1 invalid / 2 could-not-run): it schema-checks the report, confirms every adrId cites an active ADR in the log and the summary counts tally, and advises (HIGH, non-fatal) when constitutionHash no longer matches constitution.md.
