package alertgroup

import (
	"context"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// AlertGroupDetails contains data received from the alert group api.
type AlertGroupDetails struct {
	GroupId           string            `json:"group_id"`
	Anomalies         map[string]string `json:"anomalies"`
	StartCommitNumber int32             `json:"start_commit"`
	EndCommitNumber   int32             `json:"end_commit"`
}

// GetQueryParams returns the query parameters corresponding to the alert group data.
func (alertGroup *AlertGroupDetails) GetQueryParams(ctx context.Context) map[string]util.StringSet {
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
		AddToSetIfNotExists(masters_map, splits[0], parsedInfo, masters_key)
		AddToSetIfNotExists(bots_map, splits[1], parsedInfo, bots_key)
		AddToSetIfNotExists(benchmarks_map, splits[2], parsedInfo, benchmarks_key)
		AddToSetIfNotExists(tests_map, splits[3], parsedInfo, tests_key)
		AddToSetIfNotExists(subtests_1_map, splits[4], parsedInfo, subtests_1_key)
		if len(splits) > 5 {
			AddToSetIfNotExists(subtests_2_map, splits[5], parsedInfo, subtests_2_key)
		}
	}

	paramsMap := map[string]util.StringSet{}
	paramsMap["stat"] = util.NewStringSet([]string{"value"})
	paramsMap["master"] = util.NewStringSet(parsedInfo[masters_key])
	paramsMap["bot"] = util.NewStringSet(parsedInfo[bots_key])
	paramsMap["benchmark"] = util.NewStringSet(parsedInfo[benchmarks_key])
	paramsMap["test"] = util.NewStringSet(parsedInfo[tests_key])
	paramsMap["subtest_1"] = util.NewStringSet(parsedInfo[subtests_1_key])

	sub_2, ok := parsedInfo[subtests_2_key]
	if ok && len(sub_2) > 0 {
		paramsMap["subtest_2"] = util.NewStringSet(parsedInfo[subtests_2_key])
	}

	return paramsMap
}

func AddToSetIfNotExists(set util.StringSet, value string, parsedInfo map[string][]string, parsedInfoKey string) {
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
