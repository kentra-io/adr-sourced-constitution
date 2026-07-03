---
name: plan-gate
description: Reads a plan plus the active constitution.md, reasons rule-by-rule, runs `constitution guard`, and emits deviation.json citing the ADR ids a plan would violate. Invoke explicitly with /plan-gate before executing a plan.
disable-model-invocation: true
---

# plan-gate

Check a plan against this repo's constitution *before* it is executed. You read
every active rule, reason about the plan rule-by-rule, and emit a
`deviation.json` report that cites, by `ADR-NNNN`, every rule the plan would
violate. You do not edit the plan and you do not change the log — this is a
read-only gate that produces a report.

Where the platform supports it, run this in a **read-only / forked context**
(a sub-agent with no write tools): the gate should never mutate anything, and
isolating it keeps that guarantee structural.

## Inputs

- The plan: a path the human gives you (default to the plan under discussion).
- The constitution: `constitution/constitution.md`.

## Procedure

1. **Load the rules.** `cat constitution/constitution.md`. Each active rule is a
   heading with an `ADR-NNNN` in its metadata line. That id is what you cite.
2. **Reason rule-by-rule.** For *every* active ADR in the projection, decide:
   does the plan conform, or conflict? Go through all of them — a rule the plan
   simply ignores can still be violated by it. For each conflict, note the
   plan location (file + line span if you can), the `ADR-NNNN`, a one-line
   summary, a severity, and whether the fix is to change the plan (`conform`) or
   to change the rule via an ADR (`amend`).
3. **Run guard and fold it in.** `constitution guard --format json`. Any
   violation it reports is an out-of-band mutation of the log itself — add each
   as a deviation citing that ADR id (typically `CRITICAL`, recommendation
   `conform`), so the report is one place the human looks.
4. **Write `deviation.json`.** Default path `./deviation.json`; honor an `--out`
   the human specifies (in a harness, write it into the plan's spec folder).
   The schema is fixed — see `docs/deviation.schema.json`. Shape:

   ```json
   {
     "generatedAt": "2026-07-03T14:02:00Z",
     "constitutionHash": "",
     "plan": "tasks/plan.md",
     "deviations": [
       {"id": "D-001", "adrId": "ADR-0007", "severity": "HIGH",
        "rule": "Guard modes and advisory manifest",
        "location": {"file": "tasks/plan.md", "lines": "12-40"},
        "summary": "Plan makes guard a required merge gate; the rule pins it advisory in v1.",
        "recommendation": "amend",
        "recommendationDetail": "Propose an ADR to supersede the advisory decision instead."}
     ],
     "summary": {"critical": 0, "high": 1, "medium": 0, "low": 0}
   }
   ```

   Rules for a valid report:
   - Every deviation **must** cite an `adrId` that exists in the log — the
     citation is the whole point.
   - Severity is one of `CRITICAL | HIGH | MEDIUM | LOW`; `recommendation` is
     `conform` or `amend`.
   - `summary` counts must equal the actual per-severity totals.
   - Leave `constitutionHash` empty for now; the validator fills you in (next
     step).

5. **Validate via the CLI — this is mandatory.** The CLI owns the schema; do not
   hand-check it. Run:

   ```
   constitution deviation validate ./deviation.json
   ```

   - Exit 0 = valid. If stderr shows a `constitutionHash mismatch [HIGH …]`
     advisory, copy the **expected `sha256:…` value it prints** into the
     report's `constitutionHash` field and run validate once more so it is
     clean. (The validator is the source of truth for that hash — do not compute
     it by hand.)
   - Exit 1 = invalid: the errors on stderr are precise (missing/unknown
     `adrId`, bad severity, mismatched summary counts, schema violations). Fix
     the report and re-run until it is valid.
   - Exit 2 = could not run (not a project root, or the log is unreadable);
     report that to the human rather than presenting a report.

6. **Present the result.** Show the human a short table — one row per deviation:
   `ADR-NNNN · severity · rule · recommendation · summary` — and the path to the
   validated `deviation.json`. State the verdict plainly: does the plan conform,
   or does it need changes / an amendment before it runs.

## Never

- Never edit the plan, the ADR log, or `constitution.md` from this skill.
- Never present a `deviation.json` you have not run through
  `constitution deviation validate` successfully.
- Never invent an `adrId`; only cite rules that are actually in the log.
