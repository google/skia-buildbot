package anomalies

import (
	"context"
	"fmt"
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
Gets a table of Chrome Perf Anomalies for the given Sheriff config. A Chrome Perf Sheriff config is associated
with a set of Chrome Perf Anomaly Configs. An Anomaly Config covers one or more benchmarks and defines what
kind of change in that benchmark constitutes an Anomaly (ie. a perf regression).

When a the change in a benchmark over times meets the criteria of an Anomaly Config, the system creates an
Anomaly entry in the database to mark that event.

This function returns a table of all "untriaged" (ie. no bug has yet been filed) Anomalies which are
"regressions" (ie. the change direction is undesirable), which were created for Anomaly Configs, where those
Anomly Configs are included in the given Sheriff Config.

In other words: Input "Sheriff Config" Name -> Anomaly Configs -> Output "Anomalies" Table

The format of the output table is CSV. The first row of the table data is the column names, these are:
  RevisionStart,RevisionEnd,Bot,TestSuite,Test,ChangeDirection,Delta%,AbsoluteDelta

RevisionStart: The revision used as the "baseline" for the comparison here.
RevisionEnd: The revision being compared to the "baseline".
Bot: The test runner machine configuration for this benchmark data. It will indicate the Operating System,
  and in some cases also the Device name.
TestSuite: The name of the "Test Suite" (a collection of Tests) to which the Test containing this Benchmark
  belongs.
Test: The name of the Test (the actual automated test case executed on a test runner) containing this
  Benchmark.
ChangeDirection: If a positive number then it means the value collected for the Benchmark went down over
  time, if a negative number then it means the value collected for the Benchmark went up.
Delta%: The percentage change in the value collected for the Benchmark.
AbsoluteDelta: The absolute change in the value collected for the Benchmark.
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
			var anomalyRows []string
			const anomalyRowFormatDescription = "RevisionStart,RevisionEnd,Bot,TestSuite,Test,ChangeDirection,Delta%,AbsoluteDelta\n\n"
			const anomalyRowFormat = "%d,%d,'%s','%s','%s',%f,%f,%f\n"
			anomalyRows = append(anomalyRows, anomalyRowFormatDescription)
			for _, anomaly := range (*getAnomaliesResponse).Anomalies {
				testPathPieces := strings.Split(anomaly.TestPath, "/")
				bot := testPathPieces[1]
				testsuite := testPathPieces[2]
				test := strings.Join(testPathPieces[3:], "/")
				startRevision := anomaly.StartRevision
				endRevision := anomaly.EndRevision
				direction := anomaly.MedianBeforeAnomaly - anomaly.MedianAfterAnomaly
				difference := anomaly.MedianAfterAnomaly - anomaly.MedianBeforeAnomaly
				delta := (100 * difference) / anomaly.MedianBeforeAnomaly
				absDelta := difference
				row := fmt.Sprintf(anomalyRowFormat, startRevision, endRevision, bot, testsuite, test, direction, delta, absDelta)
				anomalyRows = append(anomalyRows, row)
			}
			return mcp.NewToolResultText(strings.Join(anomalyRows, "\n")), nil
		},
	}
	return []common.Tool{getSheriffConfigNamesTool, getAnomaliesTool}
}
