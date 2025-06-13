package chromeperf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/mcp/services/perf/pinpoint"
)

// GetTools returns tools supported by Chromeperf.
func GetTools(httpClient *http.Client) []common.Tool {
	return []common.Tool{
		{
			Name:        "List Bot Configurations",
			Description: listBotConfigurationDescription,
			Arguments: []common.ToolArgument{
				// Not required, because this can either provide the entire list of bots,
				// or the subset of bots supported by the given benchmark.
				pinpoint.BenchmarkArgument(false),
			},
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				cpc := NewChromeperfClient(request.GetArguments())

				resp, err := cpc.ListBotConfigurations(ctx, httpClient)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				var b bytes.Buffer
				err = json.NewEncoder(&b).Encode(resp)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				return mcp.NewToolResultText(b.String()), nil
			},
		},
		{
			Name:        "List Benchmarks",
			Description: listBenchmarkDescription,
			Arguments:   []common.ToolArgument{},
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				cpc := NewChromeperfClient(nil)

				resp, err := cpc.ListBenchmarks(ctx, httpClient)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				var b bytes.Buffer
				err = json.NewEncoder(&b).Encode(resp)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				return mcp.NewToolResultText(b.String()), nil
			},
		},
		{
			Name:        "List Stories",
			Description: listStoryDescription,
			Arguments: []common.ToolArgument{
				pinpoint.BenchmarkArgument(true),
			},
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				cpc := NewChromeperfClient(request.GetArguments())

				resp, err := cpc.ListStories(ctx, httpClient)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				var b bytes.Buffer
				err = json.NewEncoder(&b).Encode(resp)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				return mcp.NewToolResultText(b.String()), nil
			},
		},
		{
			Name:        "Get Chart URL",
			Description: "Generate a URL to the chart for a given anomaly.",
			Arguments: []common.ToolArgument{
				{
					Name:         "anomaly_id",
					Description:  "The ID of the anomaly in question. This is a required field.",
					Required:     true,
					ArgumentType: common.StringArgument,
				},
			},
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return mcp.NewToolResultText(
					fmt.Sprintf("%s%s", "https://chrome-perf.corp.goog/u/?anomalyIDs=", request.GetArguments()["anomaly_id"].(string))), nil
			},
		},
	}
}
