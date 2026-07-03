<!--
  GENERATED FILE -- projection of the ADR log in constitution/adr/.
  Do not hand-edit; changes will be overwritten by the next "constitution
  regen". To change a rule, add, supersede, or deprecate an ADR instead.
-->

# Constitution

## process

### Mediate every ADR write through the constitution CLI

Mediate every ADR write through the constitution CLI. `constitution adr new --title … --category … --body-file <path|->` supplies the MADR body only; the CLI validates the mandatory sections, allocates the id, composes frontmatter, writes atomically, and auto-runs regen. `supersede`/`deprecate` take a target id. Agents never write into `adr/` directly; a skill drafts to a temp file and invokes the CLI only on human acceptance, so a rejected draft never touches the log.

ADR-0003 · 2026-07-03
