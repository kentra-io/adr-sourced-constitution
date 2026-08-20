package constitution

// SkillsMinCLIVersion is the lowest `constitution` version the
// bundled skills in skills/ are written for.
//
// The skills reach an agent through two independent channels — the
// plugin catalog (kentra-agentic-plugins) and this binary's
// //go:embed fan-out — and nothing detects when the two disagree
// (issue #32). Each SKILL.md states this version and tells the agent
// to check `constitution --version` before acting on anything else,
// so a stale copy fails loudly instead of quietly driving a binary
// whose flags it does not know.
//
// Bump this, .claude-plugin/plugin.json, and the skills together in
// the release commit — docs/releasing.md carries the checklist.
const SkillsMinCLIVersion = "0.3.1"
