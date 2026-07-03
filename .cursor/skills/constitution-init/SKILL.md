---
name: constitution-init
description: Conversational greenfield interview that bootstraps a project's constitution — gathers targets, consent policy, source tracking, categories, and founding principles, then seeds them all in a single `constitution init` invocation (founding ADRs via --founding-file). Invoke explicitly with /constitution-init.
disable-model-invocation: true
---

# constitution-init

Bootstrap this repository's constitution by interviewing the human, then run
`constitution init` **exactly once** with everything you gathered. This is a
conversation, not a form — ask one thing at a time, in your own words, and
confirm each answer before moving on. Run the interview in the normal
(non-forked) context so you can actually talk to the human.

First check you are not clobbering an existing setup: if `constitution.yml`
already exists, stop and tell the human this repo is already initialized (a
re-run only refreshes integration; it will not re-seed founding ADRs).

## Interview — gather, confirming each answer

1. **Agent-instruction targets.** Which files should carry the managed pointer
   to the constitution? Offer `CLAUDE.md` (gets a real `@import`, always
   recommended for Claude Code) and `AGENTS.md` (a short pointer read by most
   other tools). Default: **both**. → `--target claude --target agents`.
2. **Consent policy.** `strict` (every ADR write needs explicit human approval —
   the recommended default) or `off` (no CLI-level gate). → `--consent strict`.
3. **Source tracking.** Ask whether decisions will be traced to an issue tracker
   (GitHub issues, Jira) or not. **Note the v1 limitation honestly:** `init`
   always writes `sourceTracking.type: none`. If the human wants tracking, tell
   them you will set it *after* init by editing the `sourceTracking` block in
   `constitution.yml` (set `type:` to `github-issue` / `jira` / `generic` and an
   optional `pattern:`) — it is config, not the log, so editing it is fine.
4. **Category vocabulary.** Propose the starter list —
   `architecture, code-style, process, testing, security, data` — as the
   default. Let the human trim or extend it. If they accept the default, pass no
   `--category` flags (init uses the starter list); otherwise pass one
   `--category <name>` per chosen category.
5. **Founding principles.** These become the first ADRs — the rules the project
   is founded on. Gather them **one at a time**: ask for a principle, help the
   human phrase it as a short imperative rule, read it back, and get an explicit
   yes before recording it. Keep going until they are done. Zero is allowed
   (they can add ADRs later with `adr-draft`).

## Record the founding principles

For anything richer than a one-line rule, write the accepted principles to a
temp Markdown file (outside the repo, e.g. `/tmp/founding.md`), one per `## `
heading — the heading becomes the ADR title, the text beneath it becomes the
rule statement:

```
## Prefer boring, well-understood technology
New dependencies must clear a high bar; default to the stdlib and proven tools.

## All changes land via reviewed pull requests
No direct pushes to the main branch; every change is reviewed.
```

Pass it with `--founding-file /tmp/founding.md`. (For trivial one-liners you may
instead repeat `--principle "<rule>"`, where the text is both title and rule.)

## Show the command, then run it once

Assemble the single `constitution init` invocation and **show it to the human
before running it**, e.g.:

```
constitution init \
  --target claude --target agents \
  --consent strict \
  --founding-file /tmp/founding.md
```

Run it **exactly once**. `init` writes `constitution.yml`, seeds one founding
ADR per principle, renders `constitution/constitution.md`, writes the managed
pointer blocks, and fans the Layer-2 skills out to `.claude/`, `.agents/`, and
`.cursor/`. Do not run it a second time to "fix" something — if a flag was
wrong, tell the human what happened and adjust deliberately.

## Verify and hand off

1. Run `constitution guard` — it should report clean.
2. `cat constitution/constitution.md` and show the human the rendered
   constitution: their founding rules, grouped by category.
3. If they asked for source tracking, edit `constitution.yml`'s `sourceTracking`
   block now (see step 3) and mention that future ADRs will then require a
   `--source`.
4. Tell them how to grow it from here: the `adr-draft` skill proposes new ADRs
   (each write gated by their consent policy), and `constitution-gov` keeps the
   constitution in context during planning.

## Never

- Never run `constitution init` more than once in this flow.
- Never hand-edit `constitution/adr/` or `constitution/constitution.md`.
- Never skip showing the human the command before you run it.
