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

## Status

**DESIGN — pending review.** The buildable design lives in
[`adr-sourced-constitution.md`](./adr-sourced-constitution.md). No code yet.

## License

[MIT](./LICENSE).
