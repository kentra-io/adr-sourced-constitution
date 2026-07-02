<!--
  GENERATED FILE -- projection of the ADR log in constitution/adr/.
  Do not hand-edit; changes will be overwritten by the next "constitution
  regen". To change a rule, add, supersede, or deprecate an ADR instead.
-->

# Constitution

## architecture

### Model the constitution as an event-sourced ADR log, final form

Every ADR is a minimal-MADR-compliant record with a mutable `status`
field (spec §4.1, §5); constitution.md is the deterministic projection
of the ADRs with `status: accepted`.

ADR-0009 · 2026-06-20 · source FS-0009

## code-style

### Format all Go code with gofmt

All Go code must be formatted with gofmt; CI rejects unformatted diffs.

ADR-0002 · 2026-06-02 · source FS-0002

### Limit source lines to 100 characters

Source lines should not exceed 100 characters; the linter warns, it does
not hard-fail the build.

ADR-0010 · 2026-06-21 · source FS-0010

## process

### Require PR review before merging an ADR

Every ADR must be merged via a pull request with at least one approval.

ADR-0003 · 2026-06-03 · source FS-0003

## testing

### Require golden tests for the regen projection

Every change to the projection ships a golden fixture asserting the
exact rendered bytes.

ADR-0006 · 2026-06-11 · source FS-0006

### Prefer table-driven tests

Prefer table-driven tests with named `t.Run` subtests for anything with
more than two similar cases.

ADR-0011 · 2026-06-22 · source FS-0011

## security

### Restrict secrets to the managed vault

All secrets must be stored in the managed vault and injected at runtime;
none may be committed or passed as plain environment variables in CI
configuration.

ADR-0007 · 2026-06-12 · source FS-0007

### Require 2FA for repository administrators

All repository administrators must have two-factor authentication
enabled; org settings enforce it.

ADR-0012 · 2026-06-23 · source FS-0012

## data

### Store all timestamps in UTC

All persisted and logged timestamps must be UTC; local-time conversion
happens only at the presentation layer.

ADR-0008 · 2026-06-13 · source FS-0008
