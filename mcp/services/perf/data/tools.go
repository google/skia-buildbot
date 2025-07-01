package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/perf/go/types"
)

// toolProvider is a struct for providing tool handlers.
type toolProvider struct {
	perfURL    string
	httpClient *http.Client
}

const (
	GetPerfDataDescription = `Retrieves the performance data for the traces (also referred to as performance tests)
	specified by the query given a begin and end time. The query consists of multiple key value pairs, where the key
	is the name of the parameter and the value is a list of parameter values. For example "test":["test1", "test2"].
	The begin and end times are provided to narrow the time frame in which to search for the raw data. The response
	data is provided in JSON representation of the GetTraceDataResponse struct. This struct contains the following information.

	- commitNumbers: This is an array of commit numbers uniquely identifying the commit.
	- commitTimestamps: This is an array of commit timestamps for the corresponding commit numbers.
	- traceset: This is a map where the key is a unique identifier for a trace and the value is an array of floating point numbers.

	The way to interpret this data is as follows:
	- Each element in the value array inside a traceset specifies the value of that trace for tests run on the corresponding commit.
 inside the header.
	- All the arrays are created in such a way that we can use index positions to determine the relation between the data point and the commit for that data point.

	By default, users generally will not want to view the commit details other than the commit number in the response. A good way of representing
	the data is a table containing the timestamp, commit number and the value for each trace in the response. Do not display the raw response json to the user.
	`
	GetPerfParamsDescription = `Retrieves the available parameters that can be used to query for performance data.
The response is a JSON object where keys are parameter names and values are lists of all possible values for that parameter.

	The keys and values supplied here are to be used to form the query string for the GetPerfData tool. The parameters returned by this tool
	are the only valid combinations of keys and values available to query the data on.`
)

// GetTraceDataResponse is a struct containing the Perf data to send back to the tool caller.
type GetTraceDataResponse struct {
	TraceSet         types.TraceSet       `json:"traceset"`
	CommitNumbers    []types.CommitNumber `json:"commitNumbers"`
	CommitTimestamps []int64              `json:"commitTimestamps"`
}

// GetTools returns the tools for the perf data domain.
func GetTools(perfUrl string, httpClient *http.Client) []common.Tool {
	toolProvider := &toolProvider{
		perfURL:    perfUrl,
		httpClient: httpClient,
	}

	return []common.Tool{
		{
			Name:        "GetPerfData",
			Description: GetPerfDataDescription,
			Arguments: []common.ToolArgument{
				{
					Name:         "begin",
					ArgumentType: common.StringArgument,
					Required:     true,
					Description:  "Timestamp string that defines the start or begin time to query for the performance data. Use RFC3339 format.",
				},
				{
					Name:         "end",
					ArgumentType: common.StringArgument,
					Required:     true,
					Description:  "Timestamp string that defines the end time to query for the performance data. Use RFC3339 format.",
				},
				{
					Name:         "query",
					ArgumentType: common.StringArgument,
					Required:     true,
					Description:  "Query string which is a url escaped string defining parameter name and parameter values to query for the performance data",
				},
			},
			Handler: toolProvider.GetPerfDataHandler,
		},
		{
			Name:        "GetPerfParams",
			Description: GetPerfParamsDescription,
			Handler:     toolProvider.GetPerfParamsHandler,
		},
	}
}

// GetPerfParamsHandler is the handler for returning the paramsets for the configured perf instance.
func (t *toolProvider) GetPerfParamsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client := NewPerfClient(t.httpClient, t.perfURL)
	resp, err := client.GetParamSet(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), err
	}
	var b bytes.Buffer
	err = json.NewEncoder(&b).Encode(resp)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), err
	}
	return mcp.NewToolResultText(b.String()), nil
}

func (t *toolProvider) GetPerfDataHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	begin, err := request.RequireString("begin")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	beginTime, err := time.ParseInLocation(time.RFC3339, begin, time.UTC)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	end, err := request.RequireString("end")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	endTime, err := time.ParseInLocation(time.RFC3339, end, time.UTC)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	perfReq := GetTraceDataRequest{
		Begin: beginTime,
		End:   endTime,
		Query: query,
	}

	client := NewPerfClient(t.httpClient, t.perfURL)
	resp, err := client.GetTraceData(ctx, perfReq)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), err
	}

	responseData := GetTraceDataResponse{
		TraceSet: resp.TraceSet,
	}
	for _, h := range resp.Header {
		responseData.CommitNumbers = append(responseData.CommitNumbers, h.Offset)
		responseData.CommitTimestamps = append(responseData.CommitTimestamps, int64(h.Timestamp))
	}

	var b bytes.Buffer
	err = json.NewEncoder(&b).Encode(responseData)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), err
	}
	return mcp.NewToolResultText(b.String()), nil
}
