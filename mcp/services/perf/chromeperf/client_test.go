package chromeperf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListBenchmarks_OK(t *testing.T) {
	ctx := context.Background()
	c := NewChromeperfClient(nil)

	expectedList := []string{
		"benchmark",
		"foo",
		"bar",
	}

	var content bytes.Buffer
	err := json.NewEncoder(&content).Encode(expectedList)
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		_, err = w.Write(content.Bytes())
		require.NoError(t, err)
	}))
	c.Url = ts.URL
	defer ts.Close()

	resp, err := c.ListBenchmarks(ctx, ts.Client())

	require.NoError(t, err)
	assert.Equal(t, content.String(), resp)
}

func TestBuildDescribeAPIUrl_EmptyBenchmark_MasterChromiumPerf(t *testing.T) {
	// check that the master is set to ChromiumPerf for the
	// current implementation. we should not be supporting any other.
	// even without benchmark, it should be set for the query.
	c := NewChromeperfClient(nil)
	url := c.buildDescribeAPIUrl("")

	expected := fmt.Sprintf("%s%s?master=ChromiumPerf", c.Url, DescribeAPI)
	assert.Equal(t, url, expected)
	assert.NotContains(t, url, "benchmark")
}

func TestBuildDescribeAPIUrl_SetBenchmark(t *testing.T) {
	// check that the master is set to ChromiumPerf for the
	// current implementation. we should not be supporting any other.
	// even without benchmark, it should be set for the query.
	c := NewChromeperfClient(nil)
	url := c.buildDescribeAPIUrl("speedometer3")

	expected := fmt.Sprintf("%s%s?master=ChromiumPerf&test_suite=speedometer3", c.Url, DescribeAPI)
	assert.Equal(t, url, expected)
}

func TestListBotConfigurations_BenchmarkSet_BotsForBenchmark(t *testing.T) {
	// if the benchmark is set, this should utilize the describe api call
	// to determine the list of bots available.
	args := map[string]any{
		"benchmark": "speedometer3",
	}
	ctx := context.Background()
	c := NewChromeperfClient(args)

	expectedBotList := []string{
		"android-bot",
		"mac-bot",
		"win-bot",
	}

	// describe API returns list in "bots", and then gets restructured into
	// response struct which renames as configurations to be in line with other
	// naming conventions.
	expectedResponseFormat := map[string]any{
		"bots": expectedBotList,
	}
	var content bytes.Buffer
	err := json.NewEncoder(&content).Encode(expectedResponseFormat)
	require.NoError(t, err)

	// test server setup
	var targetUrl string
	var targetMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetUrl = r.URL.String()
		targetMethod = r.Method
		w.Header().Add("Content-Type", "application/json")
		_, err = w.Write(content.Bytes())
		require.NoError(t, err)
	}))
	// override the target URL to the test server URL
	c.Url = ts.URL

	resp, err := c.ListBotConfigurations(ctx, ts.Client())
	require.NoError(t, err)

	assert.Equal(t, resp.Configurations, expectedBotList)
	assert.Contains(t, targetUrl, "/api/describe")
	assert.Equal(t, targetMethod, "POST")
}

func TestListBotConfigurations_NoBenchmark_AllBots(t *testing.T) {
	ctx := context.Background()
	c := NewChromeperfClient(nil)

	expectedBotList := []string{
		"android-bot",
		"mac-bot",
		"win-bot",
	}
	// /api/config returns back as "configurations" instead of "bots"
	expectedResponseFormat := map[string]any{
		"configurations": expectedBotList,
	}
	var content bytes.Buffer
	err := json.NewEncoder(&content).Encode(expectedResponseFormat)
	require.NoError(t, err)

	// test server setup
	var targetUrl string
	var targetMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetUrl = r.URL.String()
		targetMethod = r.Method
		w.Header().Add("Content-Type", "application/json")
		_, err = w.Write(content.Bytes())
		require.NoError(t, err)
	}))
	// override the target URL to the test server URL
	c.Url = ts.URL

	resp, err := c.ListBotConfigurations(ctx, ts.Client())
	require.NoError(t, err)

	assert.Equal(t, resp.Configurations, expectedBotList)
	assert.Contains(t, targetUrl, "/api/config")
	assert.Equal(t, targetMethod, "POST")
}

func TestListStories_OK(t *testing.T) {
	args := map[string]any{
		"benchmark": "speedometer3",
	}
	ctx := context.Background()
	c := NewChromeperfClient(args)

	expectedStories := []string{
		"default",
	}
	expectedResponseFormat := map[string]any{
		"cases": expectedStories,
	}
	var content bytes.Buffer
	err := json.NewEncoder(&content).Encode(expectedResponseFormat)
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		_, err = w.Write(content.Bytes())
		require.NoError(t, err)
	}))
	c.Url = ts.URL

	resp, err := c.ListStories(ctx, ts.Client())
	require.NoError(t, err)

	assert.Equal(t, resp, expectedStories)
}
