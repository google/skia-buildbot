package perf

import (
	"context"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/mcp/services/perf/pinpoint"
	"go.skia.org/infra/perf/go/chromeperf"
)

type PerfService struct {
	chromePerfClient chromeperf.ChromePerfClient
}

// Initialize the service with the provided arguments.
func (s *PerfService) Init(serviceArgs string) error {
	ctx := context.Background()
	var err error
	s.chromePerfClient, err = chromeperf.NewChromePerfClient(ctx, "", true)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create chrome perf client.")
	}
	return nil
}

// Response object for the request from sheriff list UI.
type GetSheriffListResponse struct {
	SheriffList []string `json:"sheriff_list"`
	Error       string   `json:"error"`
}

const kGetSheriffDescription string = `
Gets the list of Chrome Perf Sheriff config _names_. A Chrome Perf Sheriff config is associated
with a set of Chrome Perf Anomaly Configs. Each Anomaly Config covers one or more Perf Benchmarks
and defines the quantitative change in those Benchmarks that constitute a Regression.
`

// GetTools returns the supported tools by the service.
func (s PerfService) GetTools() []common.Tool {
	return append(pinpoint.GetTools(), common.Tool{
		Name:        "GetSheriffConfigNames",
		Description: kGetSheriffDescription,
		Arguments:   []common.ToolArgument{},
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			getSheriffListResponse := &GetSheriffListResponse{}
			err := s.chromePerfClient.SendGetRequest(ctx, "sheriff_configs_skia", "", url.Values{}, getSheriffListResponse)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if getSheriffListResponse.Error != "" {
				return mcp.NewToolResultError(getSheriffListResponse.Error), nil
			}
			return mcp.NewToolResultText(strings.Join(getSheriffListResponse.SheriffList, ",")), nil
		},
	})
}
