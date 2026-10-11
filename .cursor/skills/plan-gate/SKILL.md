---
name: plan-gate
description: Checks a plan against the project's constitution rule by rule and reports, in the session, every ADR rule the plan would break. Use before a plan is executed, when a plan or design changes, or when asked whether something conforms to the constitution. Writes deviation.json only when asked.
---

# plan-gate

## Requires

This skill is written for `constitution` 0.3.1 or newer. Before doing
anything else, run `constitution --version`. If the binary is older,
**stop** and report the mismatch — an older CLI does not have the
flags this skill uses, and the workarounds an older skill taught (such
as hand-editing `constitution.yml`) are forbidden here. Do not
improvise around the gap.

Check a plan against this repo's constitution before it is executed. Read
every standing rule, decide for each one whether the plan conforms, and report
the verdict in the session, citing each broken rule by `ADR-NNNN`. You change
nothing: not the plan, not the log, not `constitution.md`.

`constitution.md` is a **curated projection**: it holds only the standing
rules — the `## Rules` of active ADRs. Point-in-time records live in
`constitution/adr/` and never appear in it, and a plan cannot deviate from
them. The rules in `constitution.md` are the complete set a plan is checked
against.

If the `constitution` binary is not on PATH, install the prebuilt release —
`curl -sSfL https://raw.githubusercontent.com/kentra-io/adr-sourced-constitution/main/install.sh | sh`
— do **not** build from source or install a Go toolchain.

## Inputs

- The plan: the path you were given, or the plan under discussion. Any format —
  markdown, YAML, a design document.
- The constitution: `constitution/constitution.md`.

## Procedure

1. **Load the rules.** `cat constitution/constitution.md`. Each standing rule is
   a `#### <slug>` entry under a `### <category>` heading, followed by the line
   naming its ADR. If it reads "No standing rules yet", every plan conforms:
   say so and stop.
2. **Decide rule by rule.** For every rule, decide whether the plan conforms or
   conflicts. Check all of them: a plan can break a rule it never mentions. For
   each conflict note the plan location (file and line span when you can), the
   `ADR-NNNN`, a one-line summary, a severity (`CRITICAL | HIGH | MEDIUM |
   LOW`), and the remedy: `conform` (change the plan) or `amend` (change the
   rule through a new ADR).
3. **Run guard.** `constitution guard --format json`. Each violation it reports
   is an out-of-band edit to the ADR log. Add it as a conflict citing that ADR,
   normally `CRITICAL` with remedy `conform`.
4. **Report the verdict in the session.** One line first — **conforms**, or
   **conflicts with N rules** — then one row per conflict:

   | ADR | Severity | Rule | Remedy | Summary |
   |---|---|---|---|---|
   | ADR-0007 | HIGH | guard-is-advisory | amend | The plan makes guard a required merge gate; the rule keeps it advisory. |

   Cite only ADRs whose rules appear in `constitution.md`.
5. **Write `deviation.json` only when asked** — by the user, or by the skill or
   tool that invoked you, which also names the path (default
   `./deviation.json`). Its schema is `docs/deviation.schema.json`:

   ```json
   {
     "generatedAt": "2026-07-03T14:02:00Z",
     "constitutionHash": "",
     "plan": "tasks/plan.md",
     "deviations": [
       {"id": "D-001", "adrId": "ADR-0007", "severity": "HIGH",
        "rule": "guard-is-advisory",
        "location": {"file": "tasks/plan.md", "lines": "12-40"},
        "summary": "The plan makes guard a required merge gate; the rule keeps it advisory.",
        "recommendation": "amend",
        "recommendationDetail": "Propose an ADR that supersedes the advisory decision."}
     ],
     "summary": {"critical": 0, "high": 1, "medium": 0, "low": 0}
   }
   ```

   Then validate it — the CLI owns the schema:
   `constitution deviation validate <path>`. Exit 0 is valid; when stderr shows
   a `constitutionHash mismatch` advisory, copy the `sha256:…` value it prints
   into `constitutionHash` and validate once more. Exit 1 lists precise errors:
   fix the report and re-run. Exit 2 means it could not run: report that
   instead of the file.

You are done when the session shows the verdict line and one row per conflict
— and, if one was asked for, a `deviation.json` that validates.

## Never

- Never edit the plan, the ADR log, or `constitution.md`.
- Never cite an ADR that does not appear in `constitution.md`.
- Never present a `deviation.json` that `constitution deviation validate` has
  not accepted.
