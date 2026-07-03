// Package constitution embeds the repo's single-source Layer-2 skill
// bundles (skills/) so `constitution init` and `constitution regen` can fan
// them out into a target repo's agent-skill trees (implementation-plan.md
// §3, §6). The same skills/ directory is also directly consumable by
// out-of-band tooling (e.g. `npx skills add kentra-io/adr-sourced-constitution`);
// embedding it makes the CLI self-contained — the fanned-out copies are real
// files, never symlinks.
//
// The embed directive must live in a package whose directory is an ancestor
// of skills/. skills/ sits at the repo root (the layout the plan pins and
// the path the npx tooling expects), so the embedding package is this root
// library package rather than an internal one; internal/scaffold consumes
// SkillsFS through it.
package constitution

import "embed"

// SkillsFS holds the skills/ tree: one skills/<name>/SKILL.md per skill. M4
// ships stubs with valid frontmatter and a placeholder body; M5 authors the
// real content.
//
//go:embed skills
var SkillsFS embed.FS
