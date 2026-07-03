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
5. **Founding principles.** These become the first ADRs. The constitution is a
   curated read model: only *rule-bearing* ADRs project into it, so for each
   principle decide with the human which of two it is:
   - A **standing rule** — a normative constraint that should govern *future*
     planning ("always X", "never Y", "prefer Z"). This is the normal case for
     a founding principle. Distill it into a 1–3 line imperative Rule statement;
     that text is what the constitution will render.
   - A **point-in-time record** — a bootstrap decision worth keeping in the log
     (a starting-point choice, an accepted tradeoff) that establishes no ongoing
     rule. It stays in the log but **does not** appear in `constitution.md`.

   Gather them **one at a time**: ask for a principle, ask "would you want an
   agent planning six months from now to be held to this?" — if yes it is a
   standing rule, so help them phrase the terse Rule statement and read it back;
   if it is just *what we decided at the start*, keep it as a record. Get an
   explicit yes before recording each. Keep going until they are done. Zero is
   allowed (they can add ADRs later with `adr-draft`).

## Record the founding principles

Write the accepted principles to a temp Markdown file (outside the repo, e.g.
`/tmp/founding.md`), one per `## ` heading — the heading becomes the ADR title,
and the text beneath it becomes the Decision Outcome (the durable record). To
make a principle a **standing rule** that projects into the constitution, give
it a nested `## Rule` subsection: its body is the exact 1–3 line rule the
constitution renders. A principle with **no** `## Rule` subsection is a
catalog-only record — it stays in the log but never appears in `constitution.md`:

```
## Prefer boring, well-understood technology
New dependencies must clear a high bar; default to the stdlib and proven tools.

## Rule
Prefer the stdlib and proven tools; a new dependency must clear a high bar.

## All changes land via reviewed pull requests
Every change is reviewed; direct pushes to the main branch are disallowed.

## Rule
All changes land via reviewed pull requests. Never push directly to main.

## Adopted Postgres as the initial datastore
Chosen at bootstrap for its maturity and the team's familiarity. (No `## Rule`,
so this is a record-only ADR: it stays in the log and will NOT appear in
constitution.md.)
```

Pass it with `--founding-file /tmp/founding.md`. (For a trivial one-line
standing rule you may instead repeat `--principle "<rule>"`, where the text is
both title and rule — every `--principle` is always rule-bearing.)

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
   constitution: their **rule-bearing** founding principles only (each `## Rule`
   they wrote), grouped by category. Record-only principles will not appear here
   — that is correct; they live in `constitution/adr/`. If every principle was a
   record-only choice, the file shows the `No standing rules yet.` placeholder,
   which is the right outcome, not a bug.
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
