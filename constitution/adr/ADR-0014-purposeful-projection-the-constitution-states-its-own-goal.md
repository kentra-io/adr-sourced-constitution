---
id: ADR-0014
title: "Purposeful projection: the constitution states its own goal"
date: 2026-07-23
status: accepted
---

## Context and Problem Statement

A rendered `constitution.md` reads as a bare list of rules grouped by
category, with nothing explaining why the document exists at all. Adopters —
and the agents consulting it during planning — lacked a frame for what the
constitution is for, which caused scope confusion during founding interviews:
decisions that belonged in a requirements doc or an ADR's own rationale got
pulled into the constitution, and vice versa.

## Decision Drivers

- The constitution's job is to hold a project's universal technical
  decisions — the "how" — so that whoever writes requirements can stay
  functional and not re-explain implementation choices in every issue.
- That purpose statement needs to travel with every rendered copy of the
  constitution, not live only in a skill or a doc a reader might not have
  open.

## Considered Options

- Leave framing to skills and docs only: no change to the rendered file
  itself; readers who haven't read the skill still see a bare rule list.
- Render a fixed purpose preamble into every `constitution.md`, right after
  the `# Constitution` heading.

## Decision Outcome

Chosen option: render a fixed preamble into every `constitution.md`. The
preamble states that the document is the source of truth for the project's
standing technical decisions — how recurring problems are solved
(architecture, mapping, testing, process) — so that requirements can stay
functional and need not re-explain implementation choices. The same text
anchors the spec's purpose section and the governance skills, so all three
surfaces agree on why the document exists. Existing repos pick up the
preamble automatically on their next `regen`.

## Consequences

Every projection now self-describes its purpose, regardless of which skill
or doc the reader has open. The preamble is template text baked into the
renderer, not an ADR-authored rule, so it is uniform across every adopting
repo and is not itself something a per-repo ADR can edit or override.
