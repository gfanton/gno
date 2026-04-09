package logengine

import (
	"cmp"
	"regexp"
	"slices"
	"time"

	"github.com/tidwall/gjson"
)

// DedupGroup holds a cluster of log lines sharing the same templatized message.
type DedupGroup struct {
	Template  string `json:"template"`
	Count     int    `json:"count"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
	Sample    string `json:"sample"`
}

// DedupResult holds the output of deduplication.
type DedupResult struct {
	Groups       []DedupGroup `json:"groups"`
	TotalMatches int          `json:"total_matches"`
	UniqueGroups int          `json:"unique_groups"`
}

var (
	reHex     = regexp.MustCompile(`[0-9a-fA-F]{12,}`)
	reIP      = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	reNumbers = regexp.MustCompile(`\d{4,}`)
)

// Templatize replaces variable parts of a message (hex strings, IPs, long
// numbers) with "*" to produce a grouping key.
func Templatize(msg string) string {
	msg = reHex.ReplaceAllString(msg, "*")
	msg = reIP.ReplaceAllString(msg, "*")
	msg = reNumbers.ReplaceAllString(msg, "*")
	return msg
}

// extractMessage pulls the human-readable message from a JSON log line.
func extractMessage(line string) string {
	if v := gjson.Get(line, "msg"); v.Exists() {
		return v.String()
	}
	if v := gjson.Get(line, "message"); v.Exists() {
		return v.String()
	}
	return line
}

// formatNanos formats nanosecond timestamps as RFC3339 UTC strings.
// Returns empty string for zero values.
func formatNanos(ns int64) string {
	if ns == 0 {
		return ""
	}
	return time.Unix(0, ns).UTC().Format(time.RFC3339)
}

// Deduplicate groups log entries by templatized message and returns
// aggregate counts sorted by first_seen (earliest first).
func Deduplicate(entries []LogEntry) *DedupResult {
	type accumulator struct {
		template  string
		count     int
		firstSeen int64
		lastSeen  int64
		sample    string
	}

	byTemplate := make(map[string]*accumulator)

	for _, e := range entries {
		msg := extractMessage(e.Line)
		tmpl := Templatize(msg)

		acc, ok := byTemplate[tmpl]
		if !ok {
			acc = &accumulator{
				template:  tmpl,
				firstSeen: e.Timestamp,
				lastSeen:  e.Timestamp,
				sample:    e.Line,
			}
			byTemplate[tmpl] = acc
		}
		acc.count++
		if e.Timestamp != 0 {
			if acc.firstSeen == 0 || e.Timestamp < acc.firstSeen {
				acc.firstSeen = e.Timestamp
			}
			if e.Timestamp > acc.lastSeen {
				acc.lastSeen = e.Timestamp
			}
		}
	}

	groups := make([]DedupGroup, 0, len(byTemplate))
	for _, acc := range byTemplate {
		groups = append(groups, DedupGroup{
			Template:  acc.template,
			Count:     acc.count,
			FirstSeen: formatNanos(acc.firstSeen),
			LastSeen:  formatNanos(acc.lastSeen),
			Sample:    acc.sample,
		})
	}

	slices.SortFunc(groups, func(a, b DedupGroup) int {
		return cmp.Compare(a.FirstSeen, b.FirstSeen)
	})

	return &DedupResult{
		Groups:       groups,
		TotalMatches: len(entries),
		UniqueGroups: len(groups),
	}
}
