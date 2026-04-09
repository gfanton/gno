package mcp

import (
	"context"
	"fmt"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gnolang/gno/misc/gnodig/internal/driver"
	"github.com/gnolang/gno/misc/gnodig/internal/logengine"
)

// ---- Input Structs

type logsSearchInput struct {
	Target        string `json:"target" jsonschema:"Log source URI (e.g. file://path/to/log.jsonl),required"`
	Text          string `json:"text,omitempty" jsonschema:"Text substring to search for"`
	Field         string `json:"field,omitempty" jsonschema:"JSON field name for field match"`
	Value         string `json:"value,omitempty" jsonschema:"Value to match when field is set"`
	Level         string `json:"level,omitempty" jsonschema:"Minimum log level: debug, info, warn, error"`
	Module        string `json:"module,omitempty" jsonschema:"Include only lines from this module"`
	ExcludeModule string `json:"exclude_module,omitempty" jsonschema:"Exclude lines from this module"`
	TimeFrom      string `json:"time_from,omitempty" jsonschema:"Start time (RFC3339 or nanoseconds)"`
	TimeTo        string `json:"time_to,omitempty" jsonschema:"End time (RFC3339 or nanoseconds)"`
	Limit         int    `json:"limit,omitempty" jsonschema:"Max results (default 50, max 200)"`
	Deduplicate   bool   `json:"deduplicate,omitempty" jsonschema:"Group identical messages and return counts"`
}

type logsSummaryInput struct {
	Target string `json:"target" jsonschema:"Log source URI,required"`
}

type logsNavigateInput struct {
	Target string `json:"target" jsonschema:"Log source URI,required"`
	Time   string `json:"time,omitempty" jsonschema:"Seek to time (RFC3339 or nanoseconds)"`
	Offset *int64 `json:"offset,omitempty" jsonschema:"Byte offset; use next_offset from previous call"`
	Count  int    `json:"count,omitempty" jsonschema:"Lines to read (default 20, max 100)"`
}

// ---- Helpers

// openSource resolves a URI and builds/caches an index.
func openSource(
	ctx context.Context,
	uri string,
	resolvers map[string]driver.Resolver,
	cache *logengine.Cache,
	cfg ServerConfig,
) (driver.LogSource, *logengine.Index, error) {
	src, err := driver.ResolveURI(uri, resolvers)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve %q: %w", uri, err)
	}
	idx, err := cache.GetOrBuild(ctx, src, cfg.ScanConfig)
	if err != nil {
		src.Close()
		return nil, nil, fmt.Errorf("build index for %q: %w", uri, err)
	}
	return src, idx, nil
}

// ---- Registration

func registerLogTools(
	srv *sdkmcp.Server,
	cache *logengine.Cache,
	resolvers map[string]driver.Resolver,
	cfg ServerConfig,
) {
	// ---- logs_search
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "logs_search",
		Description: desc("logs_search"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in logsSearchInput) (*sdkmcp.CallToolResult, any, error) {
		src, idx, err := openSource(ctx, in.Target, resolvers, cache, cfg)
		if err != nil {
			return nil, nil, err
		}
		defer src.Close()

		q := logengine.Query{
			Text:          in.Text,
			Field:         in.Field,
			Value:         in.Value,
			Module:        in.Module,
			ExcludeModule: in.ExcludeModule,
			Limit:         in.Limit,
		}
		if q.Limit == 0 {
			q.Limit = 50
		}
		if q.Limit > 200 {
			q.Limit = 200
		}
		if in.Level != "" {
			q.Level = logengine.ParseLevelName(in.Level)
		}
		if in.TimeFrom != "" {
			ts, err := logengine.ParseTimestamp(in.TimeFrom)
			if err != nil {
				return nil, nil, err
			}
			q.TimeFrom = ts
		}
		if in.TimeTo != "" {
			ts, err := logengine.ParseTimestamp(in.TimeTo)
			if err != nil {
				return nil, nil, err
			}
			q.TimeTo = ts
		}

		entries, err := logengine.Search(ctx, src, idx, q)
		if err != nil {
			return nil, nil, fmt.Errorf("search: %w", err)
		}
		if in.Deduplicate {
			return textResult(logengine.Deduplicate(entries))
		}
		return textResult(entries)
	})

	// ---- logs_summary
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "logs_summary",
		Description: desc("logs_summary"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in logsSummaryInput) (*sdkmcp.CallToolResult, any, error) {
		src, idx, err := openSource(ctx, in.Target, resolvers, cache, cfg)
		if err != nil {
			return nil, nil, err
		}
		defer src.Close()

		summary := logengine.Summarize(idx)

		fields, fieldsErr := logengine.ExtractFields(ctx, src, idx)
		metadata, metaErr := logengine.ExtractSummaryMetadata(ctx, src, idx)

		type summaryWithFields struct {
			logengine.Summary
			HeightMin         int64          `json:"height_min,omitempty"`
			HeightMax         int64          `json:"height_max,omitempty"`
			ValidatorIdentity string         `json:"validator_identity,omitempty"`
			Fields            map[string]int `json:"fields,omitempty"`
			FieldsError       string         `json:"fields_error,omitempty"`
			MetadataError     string         `json:"metadata_error,omitempty"`
		}

		out := summaryWithFields{Summary: summary}
		if metaErr != nil {
			out.MetadataError = metaErr.Error()
		} else {
			out.HeightMin = metadata.HeightMin
			out.HeightMax = metadata.HeightMax
			out.ValidatorIdentity = metadata.ValidatorIdentity
		}
		if fieldsErr != nil {
			out.FieldsError = fieldsErr.Error()
		} else {
			out.Fields = fields
		}

		return textResult(out)
	})

	// ---- logs_navigate
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "logs_navigate",
		Description: desc("logs_navigate"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in logsNavigateInput) (*sdkmcp.CallToolResult, any, error) {
		hasTime := in.Time != ""
		hasOffset := in.Offset != nil

		if hasTime == hasOffset {
			return nil, nil, fmt.Errorf("provide exactly one of time or offset")
		}

		src, idx, err := openSource(ctx, in.Target, resolvers, cache, cfg)
		if err != nil {
			return nil, nil, err
		}
		defer src.Close()

		r, _, err := src.Reader(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("open reader: %w", err)
		}

		count := in.Count
		if count <= 0 {
			count = 20
		}
		if count > 100 {
			count = 100
		}

		var startOffset int64
		if hasOffset {
			startOffset = *in.Offset
		}

		cursor := logengine.NewCursor(r, idx, startOffset)

		var warning string
		if hasTime {
			ts, err := logengine.ParseTimestamp(in.Time)
			if err != nil {
				return nil, nil, err
			}

			// Check if requested time is outside the file's range.
			if len(idx.Blocks) > 0 {
				var tsMin, tsMax int64
				for _, b := range idx.Blocks {
					if b.TsMin != 0 && (tsMin == 0 || b.TsMin < tsMin) {
						tsMin = b.TsMin
					}
					if b.TsMax > tsMax {
						tsMax = b.TsMax
					}
				}
				if ts > tsMax {
					warning = fmt.Sprintf("requested time %s is after file's last entry at %s",
						in.Time, time.Unix(0, tsMax).UTC().Format(time.RFC3339))
				} else if ts < tsMin {
					warning = fmt.Sprintf("requested time %s is before file's first entry at %s",
						in.Time, time.Unix(0, tsMin).UTC().Format(time.RFC3339))
				}
			}
			cursor.SeekTime(ts)
		}

		entries, err := cursor.Read(count)
		if err != nil {
			return nil, nil, fmt.Errorf("read: %w", err)
		}

		return textResult(logengine.NavigateResult{
			Warning:    warning,
			Entries:    entries,
			NextOffset: cursor.Offset(),
		})
	})
}
