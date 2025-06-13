package anomalies

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/chromeperf"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

// Response object for the request from sheriff list UI.
type GetSheriffListResponse struct {
	SheriffList []string `json:"sheriff_list"`
	Error       string   `json:"error"`
}

// Response object for the request from the anomaly table UI.
type GetAnomaliesResponse struct {
	Subscription *pb.Subscription `json:"subscription"`
	// List of alerts to display.
	Alerts []alerts.Alert `json:"alerts"`
	// The list of anomalies.
	Anomalies []chromeperf.Anomaly `json:"anomaly_list"`
	// The cursor of the current query. It will be used to 'Load More' for the next query.
	QueryCursor string `json:"anomaly_cursor"`
	// Error message if any.
	Error string `json:"error"`
}

const kGetSheriffConfigNamesDescription string = `
Gets the list of Chrome Perf Sheriff config _names_. A Chrome Perf Sheriff config is associated
with a set of Chrome Perf Anomaly Configs. Each Anomaly Config covers one or more Perf Benchmarks
and defines the quantitative change in those Benchmarks that constitute an Anomaly (ie. a perf regression).

This functions returns a list of Sheriff Config Names, separated by commas.
`
const kGetAnomaliesDescription string = `
Returns a list of ChromePerf Anomalies for the given Sherriff config. The perf system continuously measures
the performance of Chrome, for various benchmarks across all major platforms. Sheriff configurations
define a set of boundaries for a certain set of benchmarks, and generally map one to one with a
performance gardening rotation. When the performance measured for a Chromium git hash falls out of
the thresholds defined by the Sheriff Config, the system generates an Anomaly. Thus, a sheriff config is associated
with a set of anomaly configs (the ones that define the thresholds against a set of benchmarks).

This function returns a table of all "untriaged" (ie. no bug has yet been filed) Anomalies which are
"regressions" (ie. the change direction is undesirable), which were created for Anomaly Configs, where those
Anomly Configs are included in the given Sheriff Config.

In other words: Input "Sheriff Config" Name -> Anomaly Configs -> Output list of Anomalies

start_revision: The revision used as the "baseline" for the comparison here.
end_revision: The revision being compared to the "baseline".
bot_name: The test runner machine configuration for this benchmark data. It will indicate the Operating System,
  and in some cases also the Device name.
`
const kSheriffConfigNameArg string = "SheriffConfigName"

func GetTools(chromePerfClient *chromeperf.ChromePerfClient) []common.Tool {
	getSheriffConfigNamesTool := common.Tool{
		Name:        "GetSheriffConfigNames",
		Description: kGetSheriffConfigNamesDescription,
		Arguments:   []common.ToolArgument{},
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			getSheriffListResponse := &GetSheriffListResponse{}
			err := (*chromePerfClient).SendGetRequest(ctx, "sheriff_configs_skia", "", url.Values{}, getSheriffListResponse)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if getSheriffListResponse.Error != "" {
				return mcp.NewToolResultError(getSheriffListResponse.Error), nil
			}
			return mcp.NewToolResultText(strings.Join(getSheriffListResponse.SheriffList, ",")), nil
		},
	}
	getAnomaliesTool := common.Tool{
		Name:        "GetAnomalies",
		Description: kGetAnomaliesDescription,
		Arguments: []common.ToolArgument{
			{
				Name:        kSheriffConfigNameArg,
				Description: "The Sheriff Config name for which to get Anomalies.",
				Required:    true,
			},
		},
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			getAnomaliesResponse := &GetAnomaliesResponse{}
			sheriffConfigName := request.GetArguments()[kSheriffConfigNameArg].(string)
			if sheriffConfigName == "" {
				return nil, skerr.Fmt("Missing required argument %s", kSheriffConfigNameArg)
			}
			var queryValues url.Values = make(url.Values)
			queryValues["host"] = []string{"https://chrome-perf.corp.goog"}
			queryValues["sheriff"] = []string{sheriffConfigName}
			//queryValues["improvements"] = []string{"false"}
			//queryValues["triaged"] = []string{"false"}
			err := (*chromePerfClient).SendGetRequest(ctx, "alerts_skia", "", queryValues, getAnomaliesResponse)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if getAnomaliesResponse.Error != "" {
				return mcp.NewToolResultError(getAnomaliesResponse.Error), nil
			}

			var b bytes.Buffer
			err = json.NewEncoder(&b).Encode(getAnomaliesResponse.Anomalies)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), err
			}

			return mcp.NewToolResultText(b.String()), nil
		},
	}
	return []common.Tool{getSheriffConfigNamesTool, getAnomaliesTool}
}
