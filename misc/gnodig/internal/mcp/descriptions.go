package mcp

import (
	_ "embed"
	"strings"
)

//go:embed mcp.md
var mcpMD string

var descriptions map[string]string

func init() {
	descriptions = parseSections(mcpMD)
}

func desc(name string) string {
	s, ok := descriptions[name]
	if !ok {
		panic("mcp: missing description section: " + name)
	}
	return s
}

func parseSections(md string) map[string]string {
	sections := make(map[string]string)
	var current string
	var b strings.Builder

	flush := func() {
		if current != "" {
			sections[current] = strings.TrimSpace(b.String())
			b.Reset()
		}
	}

	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## ") {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		if current != "" {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	flush()

	return sections
}
