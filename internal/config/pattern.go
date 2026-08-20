package config

// WrapSourcePattern returns the anchored form of a
// sourceTracking.pattern that both the write-path validator
// (Config.Validate) and the CLI's source check
// (cmd/constitution/writepath.go's validateSource) compile:
// `^(?:<pattern>)$`.
//
// It exists so the two cannot build that string differently. Issue
// #23 flagged the risk as validating one form and using another;
// validity cannot actually diverge (wrapping a valid regexp in a
// non-capturing group is always valid), but anchoring can, and a
// silently unanchored source check would accept a ref that merely
// CONTAINS a match.
func WrapSourcePattern(pattern string) string {
	return "^(?:" + pattern + ")$"
}
