package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/mcp/services/skia/format"
	"google.golang.org/genai"
)

type MCPClient interface {
	CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error)
	Tools() []*genai.Tool
}

func CallToolJSON(ctx context.Context, c MCPClient, toolName string, args map[string]interface{}, result interface{}) error {
	args[format.ArgFormat] = format.FormatJSON
	res, err := c.CallTool(ctx, toolName, args)
	if err != nil {
		return skerr.Wrap(err)
	}
	var textContent strings.Builder
	for _, content := range res.Content {
		if tc, ok := content.(mcp.TextContent); ok {
			textContent.WriteString(tc.Text)
		} else {
			textContent.WriteString(fmt.Sprintf("%v", content))
		}
		textContent.WriteString("\n")
	}
	if res.IsError {
		return skerr.Fmt("tool reported an error: %s", textContent.String())
	}
	return skerr.Wrap(json.NewDecoder(bytes.NewReader([]byte(textContent.String()))).Decode(result))
}
