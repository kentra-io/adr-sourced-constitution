---
id: ADR-0011
title: Pin the byte-fidelity stack for deterministic projection
category: architecture
date: 2026-07-03
status: deprecated
---

## Context and Problem Statement

The projection must be byte-for-byte deterministic and an accepted ADR must never have its untouched bytes disturbed by a status change. These guarantees depend on specific, verified library and technique choices (implementation-plan.md §3).

## Decision Drivers

- No YAML library guarantees byte-for-byte round-trip of untouched content, so an accepted ADR cannot be rewritten by re-serializing it.
- The archived `gopkg.in/yaml.v3` needs a maintained successor used uniformly.
- Atomic writes must work on all build targets, including Windows, where `google/renameio` exports nothing.

## Considered Options

- Rewrite ADRs by re-serializing parsed YAML.
- Use the archived yaml.v3 and a Unix-only atomic-write helper.
- Pin a byte-fidelity stack: maintained YAML lib, line-targeted textual patches, hand-rolled atomic writes.

## Decision Outcome

Pin the byte-fidelity stack. Use `go.yaml.in/yaml/v3` everywhere (the maintained successor to the archived yaml.v3), but only to validate — never to rewrite an accepted ADR. Change an ADR's status with a line-targeted textual patch (find `^status:`, replace the value; insert `superseded-by:` after it) so untouched bytes are preserved exactly. Write every file atomically via a same-dir temp file plus rename (MoveFileEx on Windows). The log is truth: a crash mid-sequence always leaves a self-consistent log that regen re-derives.
