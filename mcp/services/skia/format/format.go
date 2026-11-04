// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package format

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/mcp/common"
)

const (
	ArgFormat      = "format"
	FormatJSON     = "json"
	FormatMarkdown = "markdown"
)

func FormatToolArgument() common.ToolArgument {
	return common.ToolArgument{
		Name:        ArgFormat,
		Description: fmt.Sprintf("[Optional] Output format, either %q or %q. Default %q.", FormatMarkdown, FormatJSON, FormatMarkdown),
	}
}

// FormatResponse marshals the response according to the requested format.
func FormatResponse(req mcp.CallToolRequest, resp fmt.Stringer) (*mcp.CallToolResult, error) {
	format := req.GetString(ArgFormat, FormatMarkdown)

	switch format {
	case FormatJSON:
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return mcp.NewToolResultText(string(b)), nil
	case FormatMarkdown:
		return mcp.NewToolResultText(resp.String()), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// FormatResponseWrapper wraps a tool handler and calls FormatResponse on its
// result.
func FormatResponseWrapper(fn func(context.Context, mcp.CallToolRequest) (fmt.Stringer, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Check the format arg before calling the potentially-expensive func.
		format := req.GetString(ArgFormat, FormatMarkdown)
		if format != FormatJSON && format != FormatMarkdown {
			return mcp.NewToolResultError(fmt.Sprintf("unknown format %s", format)), nil
		}
		resp, err := fn(ctx, req)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("", err), nil
		}
		return FormatResponse(req, resp)
	}
}
