package logengine_test

import (
	"testing"

	"github.com/gnolang/gno/misc/gnodig/internal/logengine"
)

func TestTemplatize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "hex replacement",
			input: "tx hash abcdef012345abcdef",
			want:  "tx hash *",
		},
		{
			name:  "IP replacement",
			input: "connected to 192.168.1.100:8080",
			want:  "connected to *:*",
		},
		{
			name:  "long number replacement",
			input: "block height 78841 processed",
			want:  "block height * processed",
		},
		{
			name:  "short numbers preserved",
			input: "got 3 peers",
			want:  "got 3 peers",
		},
		{
			name:  "combined",
			input: "peer 10.0.0.1 sent tx abcdef012345 at height 12345",
			want:  "peer * sent tx * at height *",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logengine.Templatize(tt.input)
			if got != tt.want {
				t.Errorf("Templatize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDeduplicate(t *testing.T) {
	entries := []logengine.LogEntry{
		{
			Line:      `{"ts":"2025-01-01T00:00:01Z","level":"info","msg":"block 10001 committed"}`,
			Timestamp: 1735689601000000000,
		},
		{
			Line:      `{"ts":"2025-01-01T00:00:02Z","level":"info","msg":"block 20002 committed"}`,
			Timestamp: 1735689602000000000,
		},
		{
			Line:      `{"ts":"2025-01-01T00:00:03Z","level":"error","msg":"connection lost"}`,
			Timestamp: 1735689603000000000,
		},
		{
			Line:      `{"ts":"2025-01-01T00:00:04Z","level":"info","msg":"block 30003 committed"}`,
			Timestamp: 1735689604000000000,
		},
	}

	result := logengine.Deduplicate(entries)

	if result.TotalMatches != 4 {
		t.Errorf("TotalMatches = %d, want 4", result.TotalMatches)
	}
	if result.UniqueGroups != 2 {
		t.Errorf("UniqueGroups = %d, want 2", result.UniqueGroups)
	}

	// Find the "block * committed" group.
	var blockGroup *logengine.DedupGroup
	for i := range result.Groups {
		if result.Groups[i].Template == "block * committed" {
			blockGroup = &result.Groups[i]
			break
		}
	}

	if blockGroup == nil {
		t.Fatal("expected 'block * committed' group")
	}
	if blockGroup.Count != 3 {
		t.Errorf("block group count = %d, want 3", blockGroup.Count)
	}
	if blockGroup.Sample == "" {
		t.Error("expected non-empty sample")
	}

	// Find the "connection lost" group.
	var connGroup *logengine.DedupGroup
	for i := range result.Groups {
		if result.Groups[i].Template == "connection lost" {
			connGroup = &result.Groups[i]
			break
		}
	}

	if connGroup == nil {
		t.Fatal("expected 'connection lost' group")
	}
	if connGroup.Count != 1 {
		t.Errorf("connection group count = %d, want 1", connGroup.Count)
	}

	// Verify sorted by first_seen: block group (00:00:01) should come before connection group (00:00:03).
	if len(result.Groups) >= 2 {
		if result.Groups[0].FirstSeen > result.Groups[1].FirstSeen {
			t.Errorf("groups not sorted by first_seen: %q > %q",
				result.Groups[0].FirstSeen, result.Groups[1].FirstSeen)
		}
	}
}
