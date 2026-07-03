<!--
  GENERATED FILE -- projection of the ADR log in constitution/adr/.
  Do not hand-edit; changes will be overwritten by the next "constitution
  regen". Only rule-bearing (## Rule) active ADRs project here; to change a
  rule, add, supersede, or deprecate an ADR instead.
-->

# Constitution

## architecture

### Model the constitution as an event-sourced ADR log, final form

Model the constitution as a deterministic projection of the accepted ADR
set; never hand-author constitution.md.

ADR-0009 · 2026-06-20 · source FS-0009

## code-style

### Format all Go code with gofmt

Format all Go code with gofmt; unformatted diffs must not merge.

ADR-0002 · 2026-06-02 · source FS-0002

## testing

### Require golden tests for the regen projection

Every projection change ships a byte-exact golden fixture.

ADR-0006 · 2026-06-11 · source FS-0006

### Prefer table-driven tests

Prefer table-driven tests with named subtests beyond two similar cases.

ADR-0011 · 2026-06-22 · source FS-0011

## security

### Restrict secrets to the managed vault

Store all secrets in the managed vault; never commit them or pass them as
plain CI environment variables.

ADR-0007 · 2026-06-12 · source FS-0007

### Require 2FA for repository administrators

Require two-factor authentication for all repository administrators.

ADR-0012 · 2026-06-23 · source FS-0012
