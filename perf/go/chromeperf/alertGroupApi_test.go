package chromeperf

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetQueryUrl_Valid_NoSubTest2(t *testing.T) {
	const master = "test_master"
	const bot = "test_bot"
	const benchmark = "test_benchmark"
	const test = "test_test"
	const subtest_1 = "test_subtest_1"
	alertGroupData := &AlertGroupDetails{
		GroupId:           "group_id",
		StartCommitNumber: 123,
		EndCommitNumber:   124,
		Anomalies: map[string]string{
			"anomaly1": fmt.Sprintf("%s/%s/%s/%s/%s", master, bot, benchmark, test, subtest_1),
		},
	}

	queryParams := alertGroupData.GetQueryParams(context.Background())
	assert.NotNil(t, queryParams, "Expected a non nil query params map")
	assert.Equal(t, master, queryParams["master"][0])
	assert.Equal(t, bot, queryParams["bot"][0])
	assert.Equal(t, benchmark, queryParams["benchmark"][0])
	assert.Equal(t, test, queryParams["test"][0])
	assert.Equal(t, subtest_1, queryParams["subtest_1"][0])
	assert.Empty(t, queryParams["subtest_2"])
}

func TestGetQueryUrl_Valid_SubTest2(t *testing.T) {
	const master = "test_master"
	const bot = "test_bot"
	const benchmark = "test_benchmark"
	const test = "test_test"
	const subtest_1 = "test_subtest_1"
	const subtest_2 = "test_subtest_2"
	alertGroupData := &AlertGroupDetails{
		GroupId:           "group_id",
		StartCommitNumber: 123,
		EndCommitNumber:   124,
		Anomalies: map[string]string{
			"anomaly1": fmt.Sprintf("%s/%s/%s/%s/%s/%s", master, bot, benchmark, test, subtest_1, subtest_2),
		},
	}

	queryParams := alertGroupData.GetQueryParams(context.Background())
	assert.Equal(t, master, queryParams["master"][0])
	assert.Equal(t, bot, queryParams["bot"][0])
	assert.Equal(t, benchmark, queryParams["benchmark"][0])
	assert.Equal(t, test, queryParams["test"][0])
	assert.Equal(t, subtest_1, queryParams["subtest_1"][0])
	assert.Equal(t, subtest_2, queryParams["subtest_2"][0])
}

func TestGetQueryUrl_DuplicateTestPath(t *testing.T) {
	const master = "test_master"
	const bot = "test_bot"
	const benchmark = "test_benchmark"
	const test = "test_test"
	const subtest_1 = "test_subtest_1"
	alertGroupData := &AlertGroupDetails{
		GroupId:           "group_id",
		StartCommitNumber: 123,
		EndCommitNumber:   124,
		Anomalies: map[string]string{
			"anomaly1": fmt.Sprintf("%s/%s/%s/%s/%s", master, bot, benchmark, test, subtest_1),
			"anomaly2": fmt.Sprintf("%s/%s/%s/%s/%s", master, bot, benchmark, test, subtest_1),
		},
	}

	queryParams := alertGroupData.GetQueryParams(context.Background())
	assert.Equal(t, master, queryParams["master"][0])
	assert.Equal(t, bot, queryParams["bot"][0])
	assert.Equal(t, benchmark, queryParams["benchmark"][0])
	assert.Equal(t, test, queryParams["test"][0])
	assert.Equal(t, subtest_1, queryParams["subtest_1"][0])
	assert.Empty(t, queryParams["subtest_2"])
}

func TestGetQueryUrl_MultipleBots(t *testing.T) {
	const master = "test_master"
	const bot = "test_bot"
	const benchmark = "test_benchmark"
	const test = "test_test"
	const subtest_1 = "test_subtest_1"
	const subtest_2 = "test_subtest_2"
	alertGroupData := &AlertGroupDetails{
		GroupId:           "group_id",
		StartCommitNumber: 123,
		EndCommitNumber:   124,
		Anomalies: map[string]string{
			"anomaly1": fmt.Sprintf("%s/%s/%s/%s/%s", master, bot, benchmark, test, subtest_1),
			// Add a different bot for the anomaly below
			"anomaly2": fmt.Sprintf("%s/%s2/%s/%s/%s", master, bot, benchmark, test, subtest_1),
		},
	}

	queryParams := alertGroupData.GetQueryParams(context.Background())
	assert.Equal(t, master, queryParams["master"][0])
	bots := queryParams["bot"]
	assert.Equal(t, 2, len(bots))
	assert.Equal(t, benchmark, queryParams["benchmark"][0])
	assert.Equal(t, test, queryParams["test"][0])
	assert.Equal(t, subtest_1, queryParams["subtest_1"][0])
	assert.Empty(t, queryParams["subtest_2"])
}
