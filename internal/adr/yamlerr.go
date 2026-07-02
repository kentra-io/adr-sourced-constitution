package adr

import (
	"regexp"
	"strconv"
)

// yamlLineRe extracts the 1-based line number go.yaml.in/yaml/v3 reports
// in scanner/parser error messages (e.g. "yaml: line 3: did not find
// expected ..."). That line is relative to the frontmatter block passed
// to yaml.Unmarshal, whose first line is always the file's line 2
// (frontmatterStartLine) — see the +1 below.
//
// Deliberately not part of our error *message* contract (only used to
// compute Line): the underlying library's wording can change between
// versions without affecting the message this package emits.
var yamlLineRe = regexp.MustCompile(`line (\d+)`)

// yamlErrorLine best-effort extracts an absolute file line number from a
// yaml.Unmarshal error; returns 0 (not determinable) if the error text
// doesn't carry a recognizable line reference.
func yamlErrorLine(err error) int {
	m := yamlLineRe.FindStringSubmatch(err.Error())
	if m == nil {
		return 0
	}
	n, err2 := strconv.Atoi(m[1])
	if err2 != nil {
		return 0 // \d+ overflow — treat as not determinable
	}
	return n + 1
}
