package chromeperf

import (
	"context"
	"net/url"
	"strings"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	AlertGroupAPIName = "alert_group"
	DetailsFuncName   = "details"
)

// AlertGroupDetails contains data received from the alert group api.
type AlertGroupDetails struct {
	GroupId           string            `json:"group_id"`
	Anomalies         map[string]string `json:"anomalies"`
	StartCommitNumber int32             `json:"start_commit"`
	EndCommitNumber   int32             `json:"end_commit"`
}

// AlertGroupApiClient provides an interface to interact with the alert_group api in chromeperf.
type AlertGroupApiClient interface {
	// GetAlertGroupDetails returns the alert group details for the provided group key.
	GetAlertGroupDetails(ctx context.Context, groupKey string) (*AlertGroupDetails, error)
}

// alertGroupApiClientImpl implements AlertGroupApiClient.
type alertGroupApiClientImpl struct {
	chromeperfClient           chromePerfClient
	getAlertGroupDetailsCalled metrics2.Counter
	getAlertGroupDetailsFailed metrics2.Counter
}

// GetAlertGroupDetails implements AlertGroupApiClient, returns alert group details for the provided group key.
func (client *alertGroupApiClientImpl) GetAlertGroupDetails(ctx context.Context, groupKey string) (*AlertGroupDetails, error) {
	if groupKey == "" {
		return nil, skerr.Fmt("Group key cannot be empty")
	}

	client.getAlertGroupDetailsCalled.Inc(1)
	// Call Chrome Perf API to fetch alert group details
	alertgroupResponse := AlertGroupDetails{}
	err := client.chromeperfClient.sendGetRequest(ctx, AlertGroupAPIName, DetailsFuncName, url.Values{"key": {groupKey}}, alertgroupResponse)
	if err != nil {
		client.getAlertGroupDetailsFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to call chrome perf endpoint.")
	}
	return &alertgroupResponse, nil
}

// GetQueryParams returns the query parameters corresponding to the alert group data.
func (alertGroup *AlertGroupDetails) GetQueryParams(ctx context.Context) map[string][]string {
	sklog.Infof("Start commit: %d, End commit: %d", alertGroup.StartCommitNumber, alertGroup.EndCommitNumber)

	// We do not want duplicate params, hence create maps to use as a set datastructure for each param
	masters_map := util.StringSet{}
	bots_map := util.StringSet{}
	benchmarks_map := util.StringSet{}
	tests_map := util.StringSet{}
	subtests_1_map := util.StringSet{}
	subtests_2_map := util.StringSet{}

	const masters_key = "masters"
	const bots_key = "bots"
	const benchmarks_key = "benchmarks"
	const tests_key = "tests"
	const subtests_1_key = "subtests_1"
	const subtests_2_key = "subtests_2"

	parsedInfo := map[string][]string{}

	for _, test := range alertGroup.Anomalies {
		splits := strings.Split(test, "/")
		addToSetIfNotExists(masters_map, splits[0], parsedInfo, masters_key)
		addToSetIfNotExists(bots_map, splits[1], parsedInfo, bots_key)
		addToSetIfNotExists(benchmarks_map, splits[2], parsedInfo, benchmarks_key)
		addToSetIfNotExists(tests_map, splits[3], parsedInfo, tests_key)
		addToSetIfNotExists(subtests_1_map, splits[4], parsedInfo, subtests_1_key)
		if len(splits) > 5 {
			addToSetIfNotExists(subtests_2_map, splits[5], parsedInfo, subtests_2_key)
		}
	}

	paramsMap := map[string][]string{}
	paramsMap["stat"] = []string{"value"}
	paramsMap["master"] = parsedInfo[masters_key]
	paramsMap["bot"] = parsedInfo[bots_key]
	paramsMap["benchmark"] = parsedInfo[benchmarks_key]
	paramsMap["test"] = parsedInfo[tests_key]
	paramsMap["subtest_1"] = parsedInfo[subtests_1_key]

	sub_2, ok := parsedInfo[subtests_2_key]
	if ok && len(sub_2) > 0 {
		paramsMap["subtest_2"] = parsedInfo[subtests_2_key]
	}

	return paramsMap
}

func addToSetIfNotExists(set util.StringSet, value string, parsedInfo map[string][]string, parsedInfoKey string) {
	// Check if the parsedinfo key is present in the parsed data
	if _, ok := parsedInfo[parsedInfoKey]; !ok {
		parsedInfo[parsedInfoKey] = []string{}
	}

	// Append to the set if it isn't already present
	if _, ok := set[value]; !ok {
		set[value] = true
		parsedInfo[parsedInfoKey] = append(parsedInfo[parsedInfoKey], value)
	}
}

// NewAlertGroupApiClient returns a new instance of AlertGroupApiClient
func NewAlertGroupApiClient(ctx context.Context) (AlertGroupApiClient, error) {
	cpClient, err := newChromePerfClient(ctx, "")
	if err != nil {
		return nil, err
	}

	return newAlertGroupApiClient(cpClient), nil
}

// newAlertGroupApiClient returns a new instance of AlertGroupApiClient with the given chromeperf client
func newAlertGroupApiClient(cpClient chromePerfClient) AlertGroupApiClient {
	return &alertGroupApiClientImpl{
		chromeperfClient:           cpClient,
		getAlertGroupDetailsCalled: metrics2.GetCounter("chrome_perf_get_alertgroup_details_called"),
		getAlertGroupDetailsFailed: metrics2.GetCounter("chrome_perf_get_alertgroup_details_failed"),
	}
}
