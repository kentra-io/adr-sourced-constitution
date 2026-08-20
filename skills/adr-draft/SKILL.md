---
name: adr-draft
description: Drafts a MADR decision-record body from the current conversation, decides whether it is a standing rule (gets a ## Rules section, projects into the constitution) or a point-in-time record (log only), writes it to a temp file, and on explicit human acceptance calls `constitution adr new`. Use when a decision worth recording as an ADR emerges. Does not pre-grant mutating commands — the Bash permission prompt is the consent checkpoint.
---

# adr-draft

## Requires

This skill is written for `constitution` 0.3.1 or newer. Before doing
anything else, run `constitution --version`. If the binary is older,
**stop** and report the mismatch — an older CLI does not have the
flags this skill uses, and the workarounds an older skill taught (such
as hand-editing `constitution.yml`) are forbidden here. Do not
improvise around the gap.

Use this when a decision worth governing has emerged in conversation — an
architectural choice, a convention, a policy — and it should become an ADR in
this repo's constitution. You draft the record; **the human accepts it; the CLI
writes it.** You never write into `constitution/adr/` yourself.

If the `constitution` binary is not on PATH, install the prebuilt release —
`curl -sSfL https://raw.githubusercontent.com/kentra-io/adr-sourced-constitution/main/install.sh | sh`
— do **not** build from source or install a Go toolchain.

## Settledness gate — check this before drafting

Only draft an ADR for a decision that has actually **settled**. If the
conversation is still exploring — brainstorming, sketching, thinking out loud —
drafting an ADR is premature. Signal words meaning *not settled*: "brainstorm",
"thoughts?", "how could we", "something like". If you see them (or the decision
otherwise feels tentative), ask the human to confirm it's settled before you
draft anything. Don't assume settledness from tone or momentum — ask.

## The consent checkpoint — read this first

Do **not** add `constitution adr new` (or `supersede`, `deprecate`) to any
`allowed-tools` / pre-approved-command list, and do not try to route around the
permission prompt (no wrapper scripts, no `eval`, no pre-authorization). When
you finally run the CLI, your harness will ask the human to approve that exact
command. **That prompt is the consent gate** — it is how a human, not the agent,
authorizes every change to the log. Working around it defeats the one guarantee
this system makes. If the command is denied, stop and leave the log untouched.

Under the `strict` consent policy in a non-interactive shell, the CLI's
`[y/N]` prompt always fails closed — there is no TTY to answer it. `--approve`
is the sanctioned path in that case, with the harness's own permission prompt
standing in as the human gate. This complements the rule above, it doesn't
relax it: you still never pre-approve or route around that harness prompt.

## Flow

1. **Confirm there's a decision.** One decision per ADR. If the conversation
   settled several, draft them one at a time.
2. **Decide: standing rule, or point-in-time record?** Ask the human (or judge
   from the conversation) which of the two this is:
   - A **standing rule** is a normative constraint that should govern *future*
     planning — "always X", "never Y", "prefer Z". It belongs in the
     constitution. Give it a `## Rules` section (below); it will project into
     `constitution.md`.
   - A **point-in-time record** documents a decision made now (a migration, a
     one-off choice, an accepted tradeoff) that establishes no ongoing rule. It
     belongs in the log for history, but **not** in the constitution. Omit the
     `## Rules` section; the ADR stays a catalog-only record.

   When unsure, ask: "would you want an agent planning six months from now to be
   held to this?" If yes, it is a rule; if it is just *what we did*, it is a
   record.
3. **Check for an existing rule.** `cat constitution/constitution.md`. If an
   active rule already covers this and you now disagree with it, this is a
   *supersession*, not a new record — see below. (Only rule-bearing active ADRs
   appear here; a record you cannot find may still exist in the log under
   `constitution/adr/`.)
4. **Draft the MADR body to a temp file.** Write these `##` sections, in this
   order, and nothing above the first heading:

   ```
   ## Context and Problem Statement
   <what forces a decision — the situation and the question>

   ## Decision Drivers
   <the criteria that matter: constraints, priorities, forces>

   ## Considered Options
   - <option A>
   - <option B>

   ## Decision Outcome
   <the decision and its rationale, at whatever length it needs. This no longer
   projects into the constitution — it is the durable record.>

   ## Rules            ← ONLY for a standing rule; omit for a record
   ### <category>
   #### <slug>
   <the standing rule, 1–3 lines, stated in the imperative — this is the exact
   text the constitution renders. Say what MUST/MUST NOT happen. Keep it terse;
   regen warns past 5 lines.>
   ```

   `Context and Problem Statement`, `Considered Options`, and `Decision Outcome`
   are mandatory (the CLI rejects a body missing any of them); include
   `Decision Drivers` too — it is standard MADR and makes the record legible.
   The `## Rules` section is optional and is what makes the ADR *rule-bearing*;
   under it, every rule lives in a `### <category>` subsection as a
   `#### <slug>` entry (kebab-case slug; body = the imperative rule text) — text
   directly under `## Rules` or a bare `### <category>` with no `#### <slug>` is
   rejected. One ADR can carry rules across several categories; each rule is
   permanently addressable as `ADR-NNNN/<category>/<slug>`. You may instead pass
   rules on the command line, repeatably, as `--rule "<category>/<slug>: <text>"`
   — one per rule — but never mix a `## Rules` section in the body with `--rule`
   flags; that combination is an error. Write the body to a temp path outside
   the repo tree, e.g. `/tmp/adr-draft.md`.

5. **Show the human the exact bytes that will be written**, then the command
   you intend to run. Display the temp file's literal contents — `cat
   /tmp/adr-draft.md` — and show that output verbatim. What you show **MUST**
   be the exact byte content of the file you pass to `--body-file`: the
   harness permission prompt shows only the command, not the file bytes, so
   the shown-draft==written-file property is part of the consent guarantee. If
   the human requests changes, edit the file and `cat` it again — loop until
   they accept the shown bytes — before you ever invoke the CLI. Then show the
   command:

   ```
   constitution adr new --title "<short imperative title>" --body-file /tmp/adr-draft.md
   ```

   There is no `--category` flag — categories ride on the rules themselves,
   either as `### <category>` subsections in the body's `## Rules` section or as
   the `<category>` segment of a `--rule "<category>/<slug>: <text>"` flag. Pick
   the category from the project's configured vocabulary — it's per-project
   data with no fixed enum (unlike, say, `sourceTracking.type`), so read it
   straight from `constitution.yml` (see `categories:`); reading the file is
   fine, only a hand-edit of it is not — config changes go through
   `constitution config set`, though `categories` specifically grows only via
   `--new-category` below, never `config set`;
   an unknown category is rejected unless you add `--new-category <name>`, which
   you should only do with the human's explicit say-so. If the project's
   `sourceTracking.type` is not `none`, add `--source <ref>`. For a standing
   rule, either put a `## Rules` section in the body **or** append one or more
   `--rule "<category>/<slug>: <text>"` flags — one or the other, never both.

6. **Only after the human explicitly accepts, run the command.** The harness
   permission prompt appears; the human approves; the CLI validates the body,
   allocates the id, writes the ADR atomically, and re-renders
   `constitution.md`. Show the human the created path.

7. **If the human rejects or edits:** revise the temp file and re-present, or
   **delete the temp file** and stop. A rejected draft must never reach
   `constitution/adr/` — the log stays append-only by never writing an
   unaccepted record.

## Phase awareness — draft vs sealed

Check `phase:` in `constitution.yml` before deciding how to fix a wrong ADR.

- **`phase: draft`** — the log is still a mutable working set. If the ADR you'd
  be superseding is a *recent mistake* rather than a decision the project has
  since built on, the cheap fix is `constitution adr edit <ADR-NNNN>`
  (per-facet: `--title`/`--source`/`--body-file`/`--rule`/the retirement-ref
  flags) or `constitution adr rm <ADR-NNNN>` — both consent-gated, same
  permission-prompt checkpoint as everything else. Don't reach for a
  supersession chain in draft when an edit is cheaper and just as honest.
- **`phase: sealed`** — the log is append-only forever; `edit`/`rm` are
  refused. `supersede`/`deprecate` are the only change paths from here on.

**`--body-file` replaces your rules.** `adr edit --body-file` swaps the
*entire* body, so any `## Rules` entry the replacement file does not
reproduce is **deleted** from the constitution — with no warning at any
layer. The ADR still validates, `guard` still reports clean, and the
next `regen` simply drops the rule from `constitution.md`.

For a prose-only fix (a reworded Decision Outcome, a corrected bullet),
never regenerate the body from memory:

1. Extract the stored body verbatim into a scratch file in the working
   directory (never `/tmp` — some agents run this inside a container with a
   standing rule against writing scratch there), and delete it once the edit
   is confirmed:
   `sed -n '/^## /,$p' constitution/adr/<file>.md > adr-body.scratch.md`
2. Patch only the section you mean to change.
3. Diff the two `## Rules` sections and confirm they are
   **byte-identical** before running the edit.

To change rules deliberately, use `--rule`, which replaces only the
Rules section and leaves every other section untouched.

## Retiring a specific rule vs. superseding a whole ADR

Not every fix is whole-ADR. If the new decision replaces or invalidates one or
more *specific rules* from prior ADRs — but those ADRs' other rules (or their
Decision Outcome as a record) still stand — retire just the rules on the new
ADR:

- `--supersedes-rule "ADR-NNNN/<category>/<slug>"` when this new ADR replaces
  that rule with something of its own.
- `--removes-rule "ADR-NNNN/<category>/<slug>"` when the rule is simply
  retired, with nothing replacing it.

Both are repeatable and available on `adr new` and `supersede` alike. Reach for
whole-ADR `supersede`/`deprecate` (below) only when the entire prior decision
is dead, not just one of its rules.

**Resurrection caveat:** retirement directives are honored only from the
*currently-accepted* ADR that wrote them. If you supersede an ADR that itself
retired some earlier rules, those earlier rules come back to life (they're no
longer masked) unless the new, superseding ADR re-retires them itself — `regen`
prints a warning naming anything resurrected this way, so check its output
after the write. The retired ADR's file is never touched either way; retirement
is a fold-time masking, not an edit.

## Superseding an existing rule

Same flow, but the decision replaces an active *rule* — or you're rewording it,
promoting a record to a rule, or demoting a rule to a record, all of which are
supersessions too (an accepted ADR's body, including its Rules, is frozen; the
only way to change it is a new ADR — see phase awareness above for the draft-
phase shortcut). Draft the superseding body the same way — including its
`## Rules` section (or `--rule` flags), since the replacement is itself a
standing rule — then run:

```
constitution supersede <ADR-NNNN> --body-file /tmp/adr-draft.md --title "<title>"
```

The CLI writes the new ADR, marks the old one `superseded`, links them, and
re-renders (mind the resurrection caveat above if the superseded ADR carried
retirements of its own). Deprecating a rule with no replacement is
`constitution deprecate <ADR-NNNN>`. The same permission-prompt consent gate
applies to every one of these.

## Never

- Never edit files under `constitution/adr/` or edit `constitution/constitution.md` directly.
- Never pre-approve or bypass the mutating command's permission prompt.
- Never write more than one decision into a single ADR.
