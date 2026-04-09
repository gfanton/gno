package mcp

import (
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func textResult(v any) (*sdkmcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling result: %w", err)
	}
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}},
	}, nil, nil
}

func rawResult(text string) (*sdkmcp.CallToolResult, any, error) {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
	}, nil, nil
}

func boolPtr(b bool) *bool { return &b }

var readOnlyAnnotation = &sdkmcp.ToolAnnotations{
	ReadOnlyHint:  true,
	OpenWorldHint: boolPtr(false),
}
