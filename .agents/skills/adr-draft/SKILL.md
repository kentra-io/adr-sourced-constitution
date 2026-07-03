---
name: adr-draft
description: Drafts a MADR decision-record body from the current conversation, writes it to a temp file, and on explicit human acceptance calls `constitution adr new`. Use when a decision worth recording as an ADR emerges. Does not pre-grant mutating commands — the Bash permission prompt is the consent checkpoint.
---

# adr-draft

Use this when a decision worth governing has emerged in conversation — an
architectural choice, a convention, a policy — and it should become an ADR in
this repo's constitution. You draft the record; **the human accepts it; the CLI
writes it.** You never write into `constitution/adr/` yourself.

## The consent checkpoint — read this first

Do **not** add `constitution adr new` (or `supersede`, `deprecate`) to any
`allowed-tools` / pre-approved-command list, and do not try to route around the
permission prompt (no wrapper scripts, no `eval`, no pre-authorization). When
you finally run the CLI, your harness will ask the human to approve that exact
command. **That prompt is the consent gate** — it is how a human, not the agent,
authorizes every change to the log. Working around it defeats the one guarantee
this system makes. If the command is denied, stop and leave the log untouched.

## Flow

1. **Confirm there's a decision.** One decision per ADR. If the conversation
   settled several, draft them one at a time.
2. **Check for an existing rule.** `cat constitution/constitution.md`. If an
   active ADR already covers this and you now disagree with it, this is a
   *supersession*, not a new record — see below.
3. **Draft the MADR body to a temp file.** Write exactly these four `##`
   sections, in this order, and nothing above the first heading:

   ```
   ## Context and Problem Statement
   <what forces a decision — the situation and the question>

   ## Decision Drivers
   <the criteria that matter: constraints, priorities, forces>

   ## Considered Options
   - <option A>
   - <option B>

   ## Decision Outcome
   <the decision, stated as a rule in the imperative — this is the text the
   constitution renders. Say what MUST/MUST NOT happen, and briefly why.>
   ```

   `Context and Problem Statement`, `Considered Options`, and `Decision Outcome`
   are mandatory (the CLI rejects a body missing any of them); include
   `Decision Drivers` too — it is standard MADR and makes the record legible.
   Write it to a temp path outside the repo tree, e.g. `/tmp/adr-draft.md`.

4. **Show the human the exact bytes that will be written**, then the command
   you intend to run. Display the temp file's literal contents — `cat
   /tmp/adr-draft.md` — and show that output verbatim. What you show **MUST**
   be the exact byte content of the file you pass to `--body-file`: the
   harness permission prompt shows only the command, not the file bytes, so
   the shown-draft==written-file property is part of the consent guarantee. If
   the human requests changes, edit the file and `cat` it again — loop until
   they accept the shown bytes — before you ever invoke the CLI. Then show the
   command:

   ```
   constitution adr new --title "<short imperative title>" --category <category> --body-file /tmp/adr-draft.md
   ```

   Pick `--category` from the project's configured vocabulary (see
   `constitution.yml`; an unknown category is rejected unless you add
   `--new-category`, which you should only do with the human's explicit say-so).
   If the project's `sourceTracking.type` is not `none`, add `--source <ref>`.

5. **Only after the human explicitly accepts, run the command.** The harness
   permission prompt appears; the human approves; the CLI validates the body,
   allocates the id, writes the ADR atomically, and re-renders
   `constitution.md`. Show the human the created path.

6. **If the human rejects or edits:** revise the temp file and re-present, or
   **delete the temp file** and stop. A rejected draft must never reach
   `constitution/adr/` — the log stays append-only by never writing an
   unaccepted record.

## Superseding an existing rule

Same flow, but the decision replaces an active ADR. Draft the superseding body
the same way, then run:

```
constitution supersede <ADR-NNNN> --body-file /tmp/adr-draft.md --title "<title>" --category <category>
```

The CLI writes the new ADR, marks the old one `superseded`, links them, and
re-renders. Deprecating a rule with no replacement is
`constitution deprecate <ADR-NNNN>`. The same permission-prompt consent gate
applies to every one of these.

## Never

- Never edit files under `constitution/adr/` or edit `constitution/constitution.md` directly.
- Never pre-approve or bypass the mutating command's permission prompt.
- Never write more than one decision into a single ADR.
