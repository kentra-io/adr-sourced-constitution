---
id: ADR-0004
title: Gate mutating commands behind a strict-or-off consent policy
category: process
date: 2026-07-03
status: accepted
---

## Context and Problem Statement

Every mutation of the log is a governance act that a human must authorize, but adopters differ on how strict that gate should be, and Layer 1 cannot by itself prove a human (not the agent) approved a write (implementation-plan.md §2.4; spec §13.5).

## Decision Drivers

- The harness's hard rule is that a human approves each amendment.
- The real v1 checkpoint is the agent-harness permission boundary, not a CLI honesty claim.
- Some adopters want pure tooling with no gate.

## Considered Options

- Always prompt, no bypass.
- No gate at all.
- A small policy vocabulary: strict or off, with a scripted-use bypass flag.

## Decision Outcome

Gate mutating commands behind a `strict | off` consent policy stored in config. Under `strict` (the default) every `adr new`/`supersede`/`deprecate` requires an interactive TTY confirm or `--approve` for scripted use; the shipped skills deliberately do not pre-grant these commands in `allowed-tools`, so the human approves each write at the harness permission prompt. `off` removes the CLI-level gate. Advisory/scoped/batched modes are deferred. Hard enforcement beyond the permission boundary is Phase 2.
