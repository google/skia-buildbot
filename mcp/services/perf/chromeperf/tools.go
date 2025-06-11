package chromeperf

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/mcp/common"
	pcomm "go.skia.org/infra/mcp/services/perf/common"
	"go.skia.org/infra/mcp/services/perf/pinpoint"
)

// GetTools returns tools supported by Chromeperf.
func GetTools() []common.Tool {
	return []common.Tool{
		{
			Name:        "List Bot Configurations",
			Description: "List all bots that Perf supports for Pinpoint execution.",
			Arguments: []common.ToolArgument{
				// Not required, because this can either provide the entire list of bots,
				// or the subset of bots supported by the given benchmark.
				pinpoint.BenchmarkArgument(false),
			},
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				cpc := NewChromeperfClient(request.GetArguments())
				httpClient, err := pcomm.DefaultHttpClient(ctx)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				resp, err := cpc.ListBotConfigurations(ctx, httpClient)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				b, err := json.Marshal(resp)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				return mcp.NewToolResultText(string(b)), nil
			},
		},
		{
			Name:        "List Benchmarks",
			Description: "List all benchmarks supported for a Pinpoint execution.",
			Arguments:   []common.ToolArgument{},
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				cpc := NewChromeperfClient(nil)
				httpClient, err := pcomm.DefaultHttpClient(ctx)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				resp, err := cpc.ListBenchmarks(ctx, httpClient)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				b, err := json.Marshal(resp)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				return mcp.NewToolResultText(string(b)), nil
			},
		},
		{
			Name:        "List Stories",
			Description: "List all stories available for a particular benchmark.",
			Arguments: []common.ToolArgument{
				pinpoint.BenchmarkArgument(true),
			},
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				cpc := NewChromeperfClient(request.GetArguments())

				httpClient, err := pcomm.DefaultHttpClient(ctx)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				resp, err := cpc.ListStories(ctx, httpClient)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				b, err := json.Marshal(resp)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				return mcp.NewToolResultText(string(b)), nil
			},
		},
	}
}
