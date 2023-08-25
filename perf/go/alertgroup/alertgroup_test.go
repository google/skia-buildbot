package alertgroup

import (
	"context"
	"fmt"
	"net/url"
	"strings"
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

	query_url := alertGroupData.GetQueryUrl(context.Background(), nil)
	assert.NotNil(t, query_url, "Expected a non nil query url")
	query_value := strings.Split(query_url, "=")[1]
	unescaped_query, err := url.QueryUnescape(query_value)
	assert.Nil(t, err)
	parsed_query, err := url.ParseQuery(unescaped_query)
	assert.Nil(t, err)
	assert.Equal(t, master, parsed_query.Get("master"))
	assert.Equal(t, bot, parsed_query.Get("bot"))
	assert.Equal(t, benchmark, parsed_query.Get("benchmark"))
	assert.Equal(t, test, parsed_query.Get("test"))
	assert.Equal(t, subtest_1, parsed_query.Get("subtest_1"))
	assert.Empty(t, parsed_query.Get("subtest_2"))
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

	query_url := alertGroupData.GetQueryUrl(context.Background(), nil)
	assert.NotNil(t, query_url, "Expected a non nil query url")
	query_value := strings.Split(query_url, "=")[1]
	unescaped_query, err := url.QueryUnescape(query_value)
	assert.Nil(t, err)
	parsed_query, err := url.ParseQuery(unescaped_query)
	assert.Nil(t, err)
	assert.Equal(t, master, parsed_query.Get("master"))
	assert.Equal(t, bot, parsed_query.Get("bot"))
	assert.Equal(t, benchmark, parsed_query.Get("benchmark"))
	assert.Equal(t, test, parsed_query.Get("test"))
	assert.Equal(t, subtest_1, parsed_query.Get("subtest_1"))
	assert.Equal(t, subtest_2, parsed_query.Get("subtest_2"))
}

func TestGetQueryUrl_DuplicateTestPath(t *testing.T) {
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
			"anomaly2": fmt.Sprintf("%s/%s/%s/%s/%s", master, bot, benchmark, test, subtest_1),
		},
	}

	query_url := alertGroupData.GetQueryUrl(context.Background(), nil)
	assert.NotNil(t, query_url, "Expected a non nil query url")
	query_value := strings.Split(query_url, "=")[1]
	unescaped_query, err := url.QueryUnescape(query_value)
	assert.Nil(t, err)
	parsed_query, err := url.ParseQuery(unescaped_query)
	assert.Nil(t, err)
	assert.Equal(t, master, parsed_query.Get("master"))
	assert.Equal(t, bot, parsed_query.Get("bot"))
	assert.Equal(t, benchmark, parsed_query.Get("benchmark"))
	assert.Equal(t, test, parsed_query.Get("test"))
	assert.Equal(t, subtest_1, parsed_query.Get("subtest_1"))
	assert.Empty(t, parsed_query.Get(subtest_2))
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

	query_url := alertGroupData.GetQueryUrl(context.Background(), nil)
	assert.NotNil(t, query_url, "Expected a non nil query url")
	query_value := strings.Split(query_url, "=")[1]
	unescaped_query, err := url.QueryUnescape(query_value)
	assert.Nil(t, err)
	parsed_query, err := url.ParseQuery(unescaped_query)
	assert.Nil(t, err)
	assert.Equal(t, master, parsed_query.Get("master"))
	bots := parsed_query["bot"]
	assert.Equal(t, 2, len(bots))
	assert.Equal(t, benchmark, parsed_query.Get("benchmark"))
	assert.Equal(t, test, parsed_query.Get("test"))
	assert.Equal(t, subtest_1, parsed_query.Get("subtest_1"))
	assert.Empty(t, parsed_query.Get(subtest_2))
}
