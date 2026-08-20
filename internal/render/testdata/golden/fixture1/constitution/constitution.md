<!--
  GENERATED FILE -- projection of the ADR log in constitution/adr/.
  Do not hand-edit; changes will be overwritten by the next "constitution
  regen". Only the rules (## Rules entries) of active ADRs project here; to
  change a rule, add, supersede, or deprecate an ADR instead.
-->

# Constitution

The source of truth for this project's standing technical decisions — how
recurring problems are solved (architecture, mapping, testing, process) — so
that requirements can stay functional and need not re-explain implementation
choices.

Decision log: constitution/adr/.

## architecture

### deterministic-projection

Model the constitution as a deterministic projection of the accepted ADR
set; never hand-author constitution.md.

ADR-0009 · 2026-06-20 · source FS-0009

## code-style

### gofmt

Format all Go code with gofmt; unformatted diffs must not merge.

ADR-0002 · 2026-06-02 · source FS-0002

## testing

### golden-tests

Every projection change ships a byte-exact golden fixture.

ADR-0006 · 2026-06-11 · source FS-0006

### table-driven-tests

Prefer table-driven tests with named subtests beyond two similar cases.

ADR-0011 · 2026-06-22 · source FS-0011

### named-subtests

Name every subtest case; anonymous t.Run blocks must not merge.

ADR-0011 · 2026-06-22 · source FS-0011

## security

### vault-secrets

Store all secrets in the managed vault; never commit them or pass them as
plain CI environment variables.

ADR-0007 · 2026-06-12 · source FS-0007

### require-2fa

Require two-factor authentication for all repository administrators.

ADR-0012 · 2026-06-23 · source FS-0012
