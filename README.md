# adr-sourced-constitution

A standalone, general-purpose primitive for spec-driven development (SDD).

It models a project's governing **constitution** — its principles plus its
accumulated architectural decisions (the *HOW* of the project) — as an
**event-sourced projection of an immutable ADR log**. The source of truth is an
append-only set of [MADR](https://adr.github.io/madr/)-compliant Architecture
Decision Records; the human-readable `constitution.md` is a **deterministically
rendered projection** of the active (non-superseded) ADR set — never hand-edited.
Changing the constitution means appending a (possibly superseding) ADR.

Because `constitution.md` is a plain Markdown file, it is **tool-neutral**: any
planning tool (Spec-Kit, OpenSpec, superpowers, or native agent plan modes) can
consume it. It is used three ways:

1. **Planning support** — loaded into a planning agent's context to shape design.
2. **Plan-validation gate** — plans are checked against it before code.
3. **Code validation** — a background sweep flags codebase drift *(deferred)*.

It ships as a Go CLI + agent-agnostic skills/commands + thin per-framework
adapters, integrating by default via a dedicated folder and an agent-instructions
pointer.

## Install

Prebuilt binaries ship on every [release](https://github.com/kentra-io/adr-sourced-constitution/releases)
for linux/darwin/windows × amd64/arm64. Pick the channel for your environment:

- **Linux / claudebox / CI / macOS-without-brew** — the neutral install script
  (arch-aware, checksum-verified, no toolchain, user-space by default):

  ```sh
  curl -sSfL https://raw.githubusercontent.com/kentra-io/adr-sourced-constitution/main/install.sh | sh
  # pin a version:      … | sh -s -- v0.1.0
  # choose a dir:       … | BINDIR=/usr/local/bin sh
  ```

  Default `BINDIR` is `~/.local/bin` (no root). To bake it into a container image,
  see [docs/releasing.md](./docs/releasing.md#claudebox--docker).

- **macOS host** — Homebrew cask from the tap (this is a **cask, macOS-only**; it
  does not cover Linux — use the script above there):

  ```sh
  brew install --cask kentra-io/tap/constitution
  ```

- **From source** (only when developing the primitive itself):
  `go install github.com/kentra-io/adr-sourced-constitution/cmd/constitution@latest`.

## Status

The [design](./adr-sourced-constitution.md) and [v1 build plan](./implementation-plan.md)
are implemented and shipped through v0.2 (M1–M4 merged 2026-07-23, released as
tag `v0.2.0` on 2026-08-11). A follow-on change released as `v0.3.0`
(issues #18–#20, plugin 0.3.0) replaced the founding file's original
one-ADR-per-principle grammar with a single MADR body — the same shape
`adr new --body-file` takes — so `init` now always seeds exactly one founding
ADR (`ADR-0001`); it also added a `constitution config` command group
(`config schema` to introspect the `constitution.yml` vocabulary, `config set`
to change one key atomically) so no skill ever hand-edits `constitution.yml`.
A second follow-on — an open-issue sweep — released as `v0.3.1` (plugin
0.3.1). It closed six issues outright: `config schema`'s `phase` and
`consent.policy` enums are now derived from the same validator maps that
enforce them, so the schema cannot drift (#26); `init` is now **atomic** —
the founding file is read, validated, and composed entirely in memory before
any byte is written, so a refused `--founding-file` (or an uncompilable
`--source-pattern`) leaves neither `constitution.yml` nor `constitution/`
behind (#22, #30); an uncompilable `sourceTracking.pattern` is refused on the
write path by both `init` and `config set`, instead of surfacing later at
the first `adr new --source` (#23); `guard` in a git repo with no commits now
explains the situation and names the `--no-git` escape instead of leaking
`fatal: bad revision 'HEAD'` (#25); a populated `constitution.md` now carries
the `Decision log: constitution/adr/.` pointer line right after the
preamble, matching the empty projection (#24); and the bundled skills now
assert a minimum CLI version before running (#32). Two more issues were only
partially addressed and stay open: `config set` loads a config that fails
validation leniently, so it stays repairable through the one supported
writer (#27, settable-key half only); and the `--body-file` rule-replacement
hazard is now named in `--help` and the adr-draft skill, with no behaviour
change (#31, documentation only).
The deferred **code-validation** sweep (README use #3) is the remaining
roadmap item.

## License

[MIT](./LICENSE).
