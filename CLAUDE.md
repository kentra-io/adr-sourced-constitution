# adr-sourced-constitution — agent notes

Go CLI: append-only MADR ADR log → deterministic `constitution.md` projection.

- **Binding docs:** `implementation-plan.md` (pinned decisions §2–§3) is authoritative;
  `adr-sourced-constitution.md` is the spec. Don't improvise where they've decided.
- **Domain invariant:** files under `constitution/adr/` are append-only — only
  `status:`/`superseded-by:` may change, and only via the CLI. Never hand-edit ADRs
  or `constitution.md` (generated; edit ADRs, run `regen`).
- **Smoke tests:** build once — `go build -o /tmp/adrc-test ./cmd/constitution` —
  and invoke by absolute path. Never name the binary `constitution` (the CLI creates
  a `constitution/` directory; names collide). No `go run` outside the module.
- E2E tests: testscript txtar in `cmd/constitution/testdata/script/`; goldens
  regenerate with `-update`.
- Read a file before Edit/Write-ing it — tooling rejects the call otherwise.

<!-- BEGIN adr-sourced-constitution v1 (managed — do not edit by hand; `constitution init` updates it) -->
@constitution/constitution.md
<!-- END adr-sourced-constitution v1 -->
