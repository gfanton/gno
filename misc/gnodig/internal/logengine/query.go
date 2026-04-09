package logengine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/tidwall/gjson"

	"github.com/gnolang/gno/misc/gnodig/internal/driver"
)

// Query describes search parameters for log lines.
type Query struct {
	Text          string // substring match against raw line bytes
	Field         string // JSON field name to match
	Value         string // expected value for Field
	Level         uint16 // minimum level filter (0 = any)
	Module        string // include only this module
	ExcludeModule string // exclude this module
	TimeFrom      int64  // inclusive lower bound, nanoseconds
	TimeTo        int64  // inclusive upper bound, nanoseconds (0 = unbounded)
	Limit         int    // max results (0 = unlimited)
	Context       int    // lines of context around each match (unused in v1)
}

// LogEntry is a single matched log line.
type LogEntry struct {
	Line      string `json:"line"`
	Offset    int64  `json:"offset"`
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`
}

// NavigateResult holds a page of log entries and the offset to continue from.
type NavigateResult struct {
	Warning    string     `json:"warning,omitempty"`
	Entries    []LogEntry `json:"entries"`
	NextOffset int64      `json:"next_offset"`
}

// Search scans src using idx to skip non-matching blocks, and returns all
// LogEntry values that satisfy q. It always returns a non-nil slice.
func Search(ctx context.Context, src driver.LogSource, idx *Index, q Query) ([]LogEntry, error) {
	results := make([]LogEntry, 0)

	r, _, err := src.Reader(ctx)
	if err != nil {
		return results, fmt.Errorf("open reader: %w", err)
	}

	// Pre-convert text pattern once, outside the block loop.
	var textBytes []byte
	if q.Text != "" {
		textBytes = []byte(q.Text)
	}

	for _, block := range idx.Blocks {
		if ctx.Err() != nil {
			return results, ctx.Err()
		}

		// ---- Block-level filters (cheap, index-only)
		if q.Level != 0 && !HasLevelOrAbove(block.LevelFlags, q.Level) {
			continue
		}
		if q.TimeFrom != 0 && q.TimeTo != 0 {
			if !block.OverlapsTimeRange(q.TimeFrom, q.TimeTo) {
				continue
			}
		} else if q.TimeFrom != 0 && block.TsMax < q.TimeFrom {
			continue
		} else if q.TimeTo != 0 && block.TsMin > q.TimeTo {
			continue
		}

		// ---- Line-level scan
		data := make([]byte, block.Size)
		if _, err := r.ReadAt(data, int64(block.Offset)); err != nil && err != io.EOF {
			return results, fmt.Errorf("read block at %d: %w", block.Offset, err)
		}

		lineOffset := int64(block.Offset)
		remaining := data
		for len(remaining) > 0 {
			nl := bytes.IndexByte(remaining, '\n')
			var line []byte
			if nl >= 0 {
				line = remaining[:nl]
				remaining = remaining[nl+1:]
			} else {
				line = remaining
				remaining = nil
			}

			if len(line) == 0 {
				lineOffset += int64(nl) + 1
				continue
			}

			lineStart := lineOffset
			if nl >= 0 {
				lineOffset += int64(nl) + 1
			} else {
				lineOffset += int64(len(line))
			}

			if !matchesQuery(line, q, textBytes) {
				continue
			}

			ts := ExtractTimestampNano(line)
			lvl := ExtractLevel(line)
			results = append(results, LogEntry{
				Line:      string(line),
				Offset:    lineStart,
				Timestamp: ts,
				Level:     lvl,
			})

			if q.Limit > 0 && len(results) >= q.Limit {
				return results, nil
			}
		}
	}

	return results, nil
}

// matchesQuery tests a single line against the query predicates.
// textBytes is the pre-converted []byte of q.Text (nil when q.Text is empty).
func matchesQuery(line []byte, q Query, textBytes []byte) bool {
	if len(textBytes) > 0 && !bytes.Contains(line, textBytes) {
		return false
	}

	if q.Field != "" && q.Value != "" {
		v := gjson.GetBytes(line, q.Field)
		if !v.Exists() || v.String() != q.Value {
			return false
		}
	}

	if q.Level != 0 {
		lvlStr := ExtractLevel(line)
		lvl := ParseLevel(lvlStr)
		if lvl == 0 || !HasLevelOrAbove(lvl, q.Level) {
			return false
		}
	}

	if q.Module != "" || q.ExcludeModule != "" {
		mod := ExtractModule(line)
		if q.Module != "" && mod != q.Module {
			return false
		}
		if q.ExcludeModule != "" && mod == q.ExcludeModule {
			return false
		}
	}

	if q.TimeFrom != 0 || q.TimeTo != 0 {
		ts := ExtractTimestampNano(line)
		if q.TimeFrom != 0 && ts < q.TimeFrom {
			return false
		}
		if q.TimeTo != 0 && ts > q.TimeTo {
			return false
		}
	}

	return true
}

// Summary holds aggregate statistics derived from an index.
type Summary struct {
	TotalBytes      int64          `json:"total_bytes"`
	TimeMin         string         `json:"time_min"`
	TimeMax         string         `json:"time_max"`
	BlocksWithLevel map[string]int `json:"blocks_with_level"`
	BlockCount      int            `json:"block_count"`
}

// Summarize computes aggregate statistics from idx without reading the source.
func Summarize(idx *Index) Summary {
	s := Summary{
		TotalBytes:      idx.SourceSize,
		BlockCount:      len(idx.Blocks),
		BlocksWithLevel: make(map[string]int),
	}

	var tsMin, tsMax int64
	for _, b := range idx.Blocks {
		if b.TsMin != 0 && (tsMin == 0 || b.TsMin < tsMin) {
			tsMin = b.TsMin
		}
		if b.TsMax > tsMax {
			tsMax = b.TsMax
		}

		if b.LevelFlags&LevelDebug != 0 {
			s.BlocksWithLevel["debug"]++
		}
		if b.LevelFlags&LevelInfo != 0 {
			s.BlocksWithLevel["info"]++
		}
		if b.LevelFlags&LevelWarn != 0 {
			s.BlocksWithLevel["warn"]++
		}
		if b.LevelFlags&LevelError != 0 {
			s.BlocksWithLevel["error"]++
		}
	}

	if tsMin != 0 {
		s.TimeMin = time.Unix(0, tsMin).UTC().Format(time.RFC3339Nano)
	}
	if tsMax != 0 {
		s.TimeMax = time.Unix(0, tsMax).UTC().Format(time.RFC3339Nano)
	}

	return s
}

// sampleBlockLines reads lines from the first and last blocks of the index,
// calling visit for each non-empty line.
func sampleBlockLines(ctx context.Context, src driver.LogSource, idx *Index, visit func(line []byte)) error {
	if len(idx.Blocks) == 0 {
		return nil
	}

	r, _, err := src.Reader(ctx)
	if err != nil {
		return fmt.Errorf("open reader: %w", err)
	}

	sample := []int{0}
	if len(idx.Blocks) > 1 {
		sample = append(sample, len(idx.Blocks)-1)
	}

	for _, i := range sample {
		block := idx.Blocks[i]
		data := make([]byte, block.Size)
		if _, err := r.ReadAt(data, int64(block.Offset)); err != nil && err != io.EOF {
			return fmt.Errorf("read block at %d: %w", block.Offset, err)
		}

		remaining := data
		for len(remaining) > 0 {
			nl := bytes.IndexByte(remaining, '\n')
			var line []byte
			if nl >= 0 {
				line = remaining[:nl]
				remaining = remaining[nl+1:]
			} else {
				line = remaining
				remaining = nil
			}

			if len(line) == 0 {
				continue
			}
			visit(line)
		}
	}

	return nil
}

// ExtractFields samples the first and last blocks of the source and returns a
// map of field name to occurrence count.
func ExtractFields(ctx context.Context, src driver.LogSource, idx *Index) (map[string]int, error) {
	fields := make(map[string]int)

	err := sampleBlockLines(ctx, src, idx, func(line []byte) {
		result := gjson.ParseBytes(line)
		result.ForEach(func(key, _ gjson.Result) bool {
			fields[key.String()]++
			return true
		})
	})

	return fields, err
}

// SummaryMetadata holds enrichments extracted by sampling log lines.
type SummaryMetadata struct {
	HeightMin         int64  `json:"height_min,omitempty"`
	HeightMax         int64  `json:"height_max,omitempty"`
	ValidatorIdentity string `json:"validator_identity,omitempty"`
}

// ExtractSummaryMetadata samples the first and last blocks to find height
// range and validator identity.
func ExtractSummaryMetadata(ctx context.Context, src driver.LogSource, idx *Index) (SummaryMetadata, error) {
	var m SummaryMetadata

	err := sampleBlockLines(ctx, src, idx, func(line []byte) {
		if h := ExtractHeight(line); h != 0 {
			if m.HeightMin == 0 || h < m.HeightMin {
				m.HeightMin = h
			}
			if h > m.HeightMax {
				m.HeightMax = h
			}
		}

		// Try to detect validator identity from common fields.
		if m.ValidatorIdentity == "" {
			for _, f := range []string{"validator", "moniker", "node_id"} {
				v := gjson.GetBytes(line, f)
				if v.Exists() && v.String() != "" {
					m.ValidatorIdentity = v.String()
					break
				}
			}
		}
	})

	return m, err
}
