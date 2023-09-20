package alertgroup

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/types"
)

// AlertGroupDetails contains data received from the alert group api.
type AlertGroupDetails struct {
	GroupId           string            `json:"group_id"`
	Anomalies         map[string]string `json:"anomalies"`
	StartCommitNumber int32             `json:"start_commit"`
	EndCommitNumber   int32             `json:"end_commit"`
}

// GetQueryUrl returns the query url corresponding to the alert group data.
func (alertGroup *AlertGroupDetails) GetQueryUrl(ctx context.Context, perfGit perfgit.Git) string {
	sklog.Infof("Start commit: %d, End commit: %d", alertGroup.StartCommitNumber, alertGroup.EndCommitNumber)
	queryUrl := url.Values{}
	// Create the end and begin query params based on the start and end commit numbers in the alert group
	if perfGit != nil {
		startCommit, err := perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(alertGroup.StartCommitNumber))
		if err != nil {
			sklog.Error("Error getting commit info")
		}
		endCommit, _ := perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(alertGroup.EndCommitNumber))
		queryUrl["begin"] = []string{strconv.Itoa(int(startCommit.Timestamp))}

		// We will shift the end time by a day so the graph doesn't render the anomalies right at the end
		endTime := time.Unix(endCommit.Timestamp, 0).AddDate(0, 0, 1)

		queryUrl["end"] = []string{strconv.Itoa(int(endTime.Unix()))}
	}

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

	// generate the query portion of the url
	query_portion := url.Values{}
	query_portion["stat"] = []string{"value"}
	query_portion["master"] = parsedInfo[masters_key]
	query_portion["bot"] = parsedInfo[bots_key]
	query_portion["benchmark"] = parsedInfo[benchmarks_key]
	query_portion["test"] = parsedInfo[tests_key]
	query_portion["subtest_1"] = parsedInfo[subtests_1_key]

	sub_2, ok := parsedInfo[subtests_2_key]
	if ok && len(sub_2) > 0 {
		query_portion["subtest_2"] = parsedInfo[subtests_2_key]
	}

	queryUrl["queries"] = []string{query_portion.Encode()}
	return queryUrl.Encode()
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
