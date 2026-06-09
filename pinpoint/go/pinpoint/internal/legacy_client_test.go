package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

func TestExtractErrorMessageReturnsErrorMessage(t *testing.T) {
	input := []byte(`{"error": "something went wrong"}`)
	expected := "something went wrong"
	actual := extractErrorMessage(input)
	assert.Equal(t, expected, actual)
}

func TestExtractErrorMessageReturnsRawString(t *testing.T) {
	input := []byte(`Internal Server Error`)
	expected := "Internal Server Error"
	actual := extractErrorMessage(input)
	assert.Equal(t, expected, actual)
}

func TestExtractErrorMessageReturnsRawStringWhenEmptyError(t *testing.T) {
	input := []byte(`{"error": ""}`)
	expected := `{"error": ""}`
	actual := extractErrorMessage(input)
	assert.Equal(t, expected, actual)
}

func TestExtractErrorMessageReturnsRawStringWhenNoError(t *testing.T) {
	input := []byte(`{"message": "some error"}`)
	expected := `{"message": "some error"}`
	actual := extractErrorMessage(input)
	assert.Equal(t, expected, actual)
}

func TestBuildBisectJobRequestUrlPopulatesAllFieldsForOldAnomaly(t *testing.T) {
	req := BisectJobCreateRequest{
		ComparisonMode:      "performance",
		StartGitHash:        "start_hash",
		EndGitHash:          "end_hash",
		Configuration:       "config",
		Benchmark:           "benchmark",
		Story:               "story",
		Chart:               "chart",
		Statistic:           "statistic",
		ComparisonMagnitude: "magnitude",
		Pin:                 "pin",
		Project:             "project",
		BugId:               "123",
		User:                "user",
		AlertIDs:            "456",
		TestPath:            "test_path",
	}

	builtURL := buildBisectJobRequestURL(&req, false)
	assert.Contains(t, builtURL, chromeperfLegacyBisectURL)

	parsedURL, err := url.Parse(builtURL)
	require.NoError(t, err)

	expected := url.Values{
		"comparison_mode":      []string{"performance"},
		"start_git_hash":       []string{"start_hash"},
		"end_git_hash":         []string{"end_hash"},
		"configuration":        []string{"config"},
		"benchmark":            []string{"benchmark"},
		"story":                []string{"story"},
		"chart":                []string{"chart"},
		"statistic":            []string{"statistic"},
		"comparison_magnitude": []string{"magnitude"},
		"pin":                  []string{"pin"},
		"project":              []string{"project"},
		"bug_id":               []string{"123"},
		"user":                 []string{"user"},
		"alert_ids":            []string{"456"},
		"test_path":            []string{"test_path"},
	}
	assert.Equal(t, expected, parsedURL.Query())
}

func TestBuildBisectJobRequestUrlPopulatesAllFieldsForNewAnomaly(t *testing.T) {
	req := BisectJobCreateRequest{
		ComparisonMode:      "performance",
		StartGitHash:        "start_hash",
		EndGitHash:          "end_hash",
		Configuration:       "config",
		Benchmark:           "benchmark",
		Story:               "story",
		Chart:               "chart",
		Statistic:           "statistic",
		ComparisonMagnitude: "magnitude",
		Pin:                 "pin",
		Project:             "project",
		BugId:               "123",
		User:                "user",
		AlertIDs:            "456",
		TestPath:            "test_path",
	}

	builtURL := buildBisectJobRequestURL(&req, true)
	assert.Contains(t, builtURL, chromeperfLegacyBisectURL)

	parsedURL, err := url.Parse(builtURL)
	require.NoError(t, err)

	// Alert IDs should not be present.
	expected := url.Values{
		"comparison_mode":      []string{"performance"},
		"start_git_hash":       []string{"start_hash"},
		"end_git_hash":         []string{"end_hash"},
		"configuration":        []string{"config"},
		"benchmark":            []string{"benchmark"},
		"story":                []string{"story"},
		"chart":                []string{"chart"},
		"statistic":            []string{"statistic"},
		"comparison_magnitude": []string{"magnitude"},
		"pin":                  []string{"pin"},
		"project":              []string{"project"},
		"bug_id":               []string{"123"},
		"user":                 []string{"user"},
		"test_path":            []string{"test_path"},
	}
	assert.Equal(t, expected, parsedURL.Query())
}

func TestBuildBisectJobRequestUrlPopulatesRequiredFields(t *testing.T) {
	req := BisectJobCreateRequest{}
	builtURL := buildBisectJobRequestURL(&req, false)
	parsedURL, err := url.Parse(builtURL)
	require.NoError(t, err)

	expected := url.Values{
		"bug_id":    []string{""},
		"test_path": []string{""},
	}
	assert.Equal(t, expected, parsedURL.Query())
}

type mockTransport struct {
	handler http.HandlerFunc
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	m.handler(recorder, req)
	return recorder.Result(), nil
}

func setupTestMocks(t *testing.T, handler http.HandlerFunc) *LegacyClient {
	client := httputils.DefaultClientConfig().WithoutRetries().Client()
	client.Transport = &mockTransport{
		handler: handler,
	}

	pc := &LegacyClient{
		httpClient:                 client,
		createBisectCalled:         metrics2.GetCounter("pinpoint_create_bisect_called"),
		createBisectFailed:         metrics2.GetCounter("pinpoint_create_bisect_failed"),
		createTryJobCalled:         metrics2.GetCounter("pinpoint_create_try_job_called"),
		createTryJobFailed:         metrics2.GetCounter("pinpoint_create_try_job_failed"),
		fetchJobStateCalled:        metrics2.GetCounter("pinpoint_fetch_job_state_called"),
		fetchJobStateFailed:        metrics2.GetCounter("pinpoint_fetch_job_state_failed"),
		queryJobListCalled:         metrics2.GetCounter("pinpoint_query_job_list_called"),
		queryJobListFailed:         metrics2.GetCounter("pinpoint_query_job_list_failed"),
		createPinpointTryJobCalled: metrics2.GetCounter("pinpoint_create_pinpoint_try_job_called"),
		createPinpointTryJobFailed: metrics2.GetCounter("pinpoint_create_pinpoint_try_job_failed"),
	}

	return pc
}

func TestDoPostRequest(t *testing.T) {
	t.Run("Returns parsed response on success", func(t *testing.T) {
		expectedResponseBody := []byte(
			`{"jobId": "12345", "jobUrl": "https://example.com/job/12345"}`,
		)
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/new", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(expectedResponseBody)
		})

		resp, err := pc.doPostRequest(context.Background(), pinpointLegacyNewJobURL)
		require.NoError(t, err)
		body, err := pc.readResponseBody(resp)
		require.NoError(t, err)
		assert.Equal(t, expectedResponseBody, body)
	})

	t.Run("Returns error if non-200 status code", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "Internal Server Error"}`))
		})

		resp, err := pc.doPostRequest(context.Background(), pinpointLegacyNewJobURL)
		require.NoError(t, err)
		body, err := pc.readResponseBody(resp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Internal Server Error")
		assert.Nil(t, body)
	})
}

func TestDoGetRequest(t *testing.T) {
	t.Run("Returns parsed response on success", func(t *testing.T) {
		expectedResponseBody := []byte(`{"job_id": "12345", "status": "completed"}`)
		testURL := "https://example.com/api/job/12345?o=STATE"
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/job/12345", r.URL.Path)
			assert.Equal(t, "STATE", r.URL.Query().Get("o"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(expectedResponseBody)
		})

		resp, err := pc.doGetRequest(context.Background(), testURL)
		require.NoError(t, err)
		body, err := pc.readResponseBody(resp)
		require.NoError(t, err)
		assert.Equal(t, expectedResponseBody, body)
	})

	t.Run("Returns error if non-200 status code", func(t *testing.T) {
		testURL := "https://example.com/api/job/12345?o=STATE"
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "Internal Server Error"}`))
		})

		resp, err := pc.doGetRequest(context.Background(), testURL)
		require.NoError(t, err)
		body, err := pc.readResponseBody(resp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Internal Server Error")
		assert.Nil(t, body)
	})
}

func TestBuildTryJobRequestUrlPopulatesRequiredFields(t *testing.T) {
	req := TryJobCreateRequest{
		Name:            "test-job",
		BaseGitHash:     "base_hash",
		EndGitHash:      "end_hash",
		BasePatch:       "base_patch",
		ExperimentPatch: "experiment_patch",
		Configuration:   "config",
		Benchmark:       "benchmark",
		Story:           "story",
		ExtraTestArgs:   "args",
		Repository:      "repo",
		BugId:           "123",
		User:            "user",
	}

	urlStr, err := buildTryJobRequestURL(&req)
	require.NoError(t, err)

	parsedURL, err := url.Parse(urlStr)
	require.NoError(t, err)

	expected := url.Values{
		"comparison_mode":  []string{tryJobComparisonMode},
		"name":             []string{"test-job"},
		"base_git_hash":    []string{"base_hash"},
		"end_git_hash":     []string{"end_hash"},
		"base_patch":       []string{"base_patch"},
		"experiment_patch": []string{"experiment_patch"},
		"configuration":    []string{"config"},
		"benchmark":        []string{"benchmark"},
		"story":            []string{"story"},
		"extra_test_args":  []string{"args"},
		"repository":       []string{"repo"},
		"bug_id":           []string{"123"},
		"user":             []string{"user"},
		"tags":             []string{"{\"origin\":\"skia_perf\"}"},
	}
	assert.Equal(t, expected, parsedURL.Query())
}

func TestBuildTryJobRequestUrlVerifiesMissingBenchmark(t *testing.T) {
	req := TryJobCreateRequest{
		Configuration: "config",
	}
	_, err := buildTryJobRequestURL(&req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Benchmark must be specified")
}

func TestBuildTryJobRequestUrlVerifiesMissingConfiguration(t *testing.T) {
	req := TryJobCreateRequest{
		Benchmark: "benchmark",
	}
	_, err := buildTryJobRequestURL(&req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Configuration must be specified")
}

func TestFetchJobState(t *testing.T) {
	t.Run("Returns parsed response on success", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/job/12345", r.URL.Path)
			assert.Equal(t, "STATE", r.URL.Query().Get("o"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"job_id": "12345", "status": "completed"}`))
		})

		resp, err := pc.FetchJobState(context.Background(), FetchJobStateRequest{JobID: "12345"})
		require.NoError(t, err)
		assert.Equal(
			t,
			&FetchJobStateResponse{
				JobID:  "12345",
				Status: "completed",
			}, resp,
		)
	})

	t.Run("Returns error if non-200 status code", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "Internal Server Error"}`))
		})

		resp, err := pc.FetchJobState(context.Background(), FetchJobStateRequest{JobID: "12345"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Internal Server Error")
		assert.Nil(t, resp)
	})
}

func TestExtractField(t *testing.T) {
	job := LegacyJobSummary{
		JobID:         "12345",
		Name:          "test_job_name",
		Benchmark:     "rendering.desktop",
		Configuration: "linux-perf",
		Story:         "loading_story",
		User:          "test-user@google.com",
		Status:        "completed",
		Arguments: map[string]string{
			"custom_arg": "custom_value",
			"story":      "story_in_args",
		},
	}

	t.Run("Extracts top-level struct fields successfully", func(t *testing.T) {
		assert.Equal(t, "12345", extractField(&job, "job_id"))
		assert.Equal(t, "test_job_name", extractField(&job, "name"))
		assert.Equal(t, "rendering.desktop", extractField(&job, "benchmark"))
		assert.Equal(t, "linux-perf", extractField(&job, "configuration"))
		assert.Equal(t, "loading_story", extractField(&job, "story"))
		assert.Equal(t, "test-user@google.com", extractField(&job, "user"))
	})

	t.Run("Case insensitive and trims spaces from input fieldName", func(t *testing.T) {
		assert.Equal(t, "12345", extractField(&job, "  JOB_ID  "))
		assert.Equal(t, "12345", extractField(&job, "Job_Id"))
	})

	t.Run("Falls back to arguments map if field is not at top-level", func(t *testing.T) {
		assert.Equal(t, "custom_value", extractField(&job, "custom_arg"))
	})

	t.Run("Falls back to arguments map if top-level field is empty", func(t *testing.T) {
		jobWithEmptyStory := LegacyJobSummary{
			Arguments: map[string]string{
				"story": "story_in_args",
			},
		}
		assert.Equal(t, "story_in_args", extractField(&jobWithEmptyStory, "story"))
	})

	t.Run("Returns empty string if field is not found", func(t *testing.T) {
		assert.Equal(t, "", extractField(&job, "non_existent_field"))
	})
}

func TestParseJobStatus(t *testing.T) {
	assert.Equal(t, pb.JobStatus_JOB_STATUS_QUEUED, parseJobStatus("queued"))
	assert.Equal(t, pb.JobStatus_JOB_STATUS_QUEUED, parseJobStatus("  Queued  "))
	assert.Equal(t, pb.JobStatus_JOB_STATUS_RUNNING, parseJobStatus("running"))
	assert.Equal(t, pb.JobStatus_JOB_STATUS_COMPLETED, parseJobStatus("completed"))
	assert.Equal(t, pb.JobStatus_JOB_STATUS_FAILED, parseJobStatus("failed"))
	assert.Equal(t, pb.JobStatus_JOB_STATUS_CANCELLED, parseJobStatus("cancelled"))
	assert.Equal(t, pb.JobStatus_JOB_STATUS_UNSPECIFIED, parseJobStatus("unknown"))
	assert.Equal(t, pb.JobStatus_JOB_STATUS_UNSPECIFIED, parseJobStatus(""))
}

func TestParseJobType(t *testing.T) {
	assert.Equal(t, pb.JobType_JOB_TYPE_TRY, parseJobType("try"))
	assert.Equal(t, pb.JobType_JOB_TYPE_TRY, parseJobType("  Try  "))
	assert.Equal(t, pb.JobType_JOB_TYPE_BISECT, parseJobType("performance"))
	assert.Equal(t, pb.JobType_JOB_TYPE_BISECT, parseJobType("functional"))
	assert.Equal(t, pb.JobType_JOB_TYPE_UNSPECIFIED, parseJobType("unknown"))
	assert.Equal(t, pb.JobType_JOB_TYPE_UNSPECIFIED, parseJobType(""))
}

func TestBuildQueryJobListParams(t *testing.T) {
	t.Run("Empty Request", func(t *testing.T) {
		req := &pb.QueryJobListRequest{}
		params := buildQueryJobListParams(req)
		assert.Empty(t, params)
	})

	t.Run("All fields populated", func(t *testing.T) {
		req := &pb.QueryJobListRequest{
			User:          "test-user@google.com",
			Configuration: "linux-perf",
			JobType:       pb.JobType_JOB_TYPE_TRY,
			Pagination: &pb.Pagination{
				PrevCursor: "prev_token",
				NextCursor: "next_token",
			},
		}
		params := buildQueryJobListParams(req)
		assert.Equal(
			t,
			[]string{
				"user=test-user@google.com",
				"configuration=linux-perf",
				"comparison_mode=try",
			},
			params["filter"],
		)
		assert.Equal(t, "prev_token", params.Get("prev_cursor"))
		assert.Equal(t, "next_token", params.Get("next_cursor"))
	})

	t.Run("Bisect JobType filter", func(t *testing.T) {
		req := &pb.QueryJobListRequest{
			JobType: pb.JobType_JOB_TYPE_BISECT,
		}
		params := buildQueryJobListParams(req)
		assert.Equal(t, []string{"comparison_mode=performance"}, params["filter"])
	})
}

func TestBuildQueryJobListRequestURL(t *testing.T) {
	t.Run("No params", func(t *testing.T) {
		req := &pb.QueryJobListRequest{}
		urlStr := buildQueryJobListRequestURL(req)
		assert.Equal(t, pinpointLegacyJobsURL, urlStr)
	})

	t.Run("With params", func(t *testing.T) {
		req := &pb.QueryJobListRequest{
			User: "test-user@google.com",
		}
		urlStr := buildQueryJobListRequestURL(req)
		parsedURL, err := url.Parse(urlStr)
		require.NoError(t, err)
		assert.Equal(t, "user=test-user@google.com", parsedURL.Query().Get("filter"))
	})
}

func TestParseQueryJobListResponse(t *testing.T) {
	t.Run("Successfully parses response with valid values and pagination", func(t *testing.T) {
		legacyResp := &LegacyQueryJobListResponse{
			Jobs: []LegacyJobSummary{
				{
					JobID:          "11111",
					Name:           "Job 1",
					Benchmark:      "blink_perf",
					Configuration:  "mac-m1",
					Story:          "scroll_story",
					User:           "user1@google.com",
					Created:        "2026-05-05T12:30:45.123456",
					Status:         "completed",
					ComparisonMode: "try",
				},
			},
			PrevCursor: "prev_cursor_token",
			NextCursor: "next_cursor_token",
			Prev:       true,
			Next:       false,
		}

		parsed := parseQueryJobListResponse(legacyResp)

		assert.NotNil(t, parsed)
		assert.Equal(t, "prev_cursor_token", parsed.Pagination.PrevCursor)
		assert.Equal(t, "next_cursor_token", parsed.Pagination.NextCursor)
		assert.True(t, *parsed.Pagination.HasPrev)
		assert.False(t, *parsed.Pagination.HasNext)

		assert.Len(t, parsed.Jobs, 1)
		j1 := parsed.Jobs[0]
		assert.Equal(t, "11111", j1.JobId)
		assert.Equal(t, "Job 1", j1.Name)
		assert.Equal(t, "blink_perf", j1.Benchmark)
		assert.Equal(t, "mac-m1", j1.Configuration)
		assert.Equal(t, "scroll_story", j1.Story)
		assert.Equal(t, "user1@google.com", j1.User)
		assert.Equal(t, pb.JobStatus_JOB_STATUS_COMPLETED, j1.JobStatus)
		assert.Equal(t, pb.JobType_JOB_TYPE_TRY, j1.JobType)

		expectedTime, err := time.Parse(legacyCreatedTimeLayout, "2026-05-05T12:30:45.123456")
		require.NoError(t, err)
		assert.Equal(t, timestamppb.New(expectedTime).AsTime(), j1.Created.AsTime())
	})

	t.Run("Handles invalid date format and fallback fields gracefully", func(t *testing.T) {
		legacyResp := &LegacyQueryJobListResponse{
			Jobs: []LegacyJobSummary{
				{
					JobID:          "22222",
					Status:         "running",
					ComparisonMode: "performance",
					Created:        "invalid-date-format",
				},
			},
		}

		parsed := parseQueryJobListResponse(legacyResp)

		assert.NotNil(t, parsed)
		assert.Len(t, parsed.Jobs, 1)
		j2 := parsed.Jobs[0]
		assert.Equal(t, "22222", j2.JobId)
		assert.Equal(t, pb.JobStatus_JOB_STATUS_RUNNING, j2.JobStatus)
		assert.Equal(t, pb.JobType_JOB_TYPE_BISECT, j2.JobType)
		assert.Nil(t, j2.Created) // invalid-date-format should be parsed as nil
		assert.Nil(t, j2.BugId)
	})

	t.Run("Parses bug_id correctly from different types", func(t *testing.T) {
		bugIDInput := int64(12345)
		expectedBugID := int64(12345)

		testCases := []struct {
			bugID    *int64
			expected *int64
			name     string
		}{
			{
				bugID:    &bugIDInput,
				expected: &expectedBugID,
				name:     "integer bug_id",
			},
			{
				bugID:    nil,
				expected: nil,
				name:     "nil bug_id",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				legacyResp := &LegacyQueryJobListResponse{
					Jobs: []LegacyJobSummary{
						{
							JobID: "1",
							BugID: tc.bugID,
						},
					},
				}
				parsed := parseQueryJobListResponse(legacyResp)
				assert.Len(t, parsed.Jobs, 1)
				if tc.expected == nil {
					assert.Nil(t, parsed.Jobs[0].BugId)
				} else {
					assert.NotNil(t, parsed.Jobs[0].BugId)
					assert.Equal(t, *tc.expected, *parsed.Jobs[0].BugId)
				}
			})
		}
	})
}

func TestQueryJobList(t *testing.T) {
	t.Run("Returns parsed bisect job on success", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/jobs", r.URL.Path)
			assert.Equal(t, "user=test-user@google.com", r.URL.Query().Get("filter"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
			  "jobs": [
			    {
			      "job_id": "10cbda9e890000",
			      "configuration": "win-10-perf",
			      "results_url": "/api/results2-serve/10cbda9e890000",
			      "improvement_direction": 1,
			      "arguments": {
				"comparison_mode": "performance",
				"target": "performance_test_suite",
				"start_git_hash": "1609167",
				"end_git_hash": "1609170",
				"grouping_label": "browse_news",
				"trace": "browse:news:reddit:2020",
				"tags": "{\"test_path\": \"ChromiumPerf/...\"}",
				"performance": "on",
				"initial_attempt_count": "20",
				"configuration": "win-10-perf",
				"benchmark": "v8.browsing_desktop",
				"story": "browse:news:reddit:2020",
				"story_tags": "",
				"chart": "Optimize:duration",
				"statistic": "avg",
				"comparison_magnitude": "71.93350000000001",
				"extra_test_args": "",
				"commit": "on,on",
				"pin": "",
				"project": "chromium",
				"bug_id": "503855894",
				"batch_id": ""
			      },
			      "bug_id": 503855894,
			      "project": "chromium",
			      "comparison_mode": "performance",
			      "name": "Performance bisect on win-10-perf/v8.browsing_desktop",
			      "user": "maximsheshukov@google.com",
			      "created": "2026-04-22T13:24:39.368547",
			      "updated": "2026-04-25T16:40:34.513983",
			      "started_time": "2026-04-24T15:04:08.027027",
			      "difference_count": 2,
			      "exception": null,
			      "status": "Completed",
			      "cancel_reason": null,
			      "batch_id": "b7d84490-b89c-4dcf-aa5f-bc14417e419c",
			      "bots": [
				"win-187-e504"
			      ]
			    }
			  ],
			  "count": 10,
			  "max_count": 1000,
			  "prev_cursor": "",
			  "next_cursor": "CjwKFAoHY3JlYXRlZBIJCNzq3t68_JMDEiBqDHN-Y2hyb21lcGVyZnIQCxIDSm9iGICApIz3pLEIDBgAIAE=",
			  "prev": false,
			  "next": true
			}`))
		})

		resp, err := pc.QueryJobList(context.Background(), &pb.QueryJobListRequest{
			User: "test-user@google.com",
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "", resp.Pagination.PrevCursor)
		assert.Equal(
			t,
			"CjwKFAoHY3JlYXRlZBIJCNzq3t68_JMDEiBqDHN-Y2hyb21lcGVyZnIQCxIDSm9iGICApIz3pLEIDBgAIAE=",
			resp.Pagination.NextCursor,
		)
		assert.False(t, *resp.Pagination.HasPrev)
		assert.True(t, *resp.Pagination.HasNext)
		assert.Len(t, resp.Jobs, 1)

		j1 := resp.Jobs[0]
		assert.Equal(t, "10cbda9e890000", j1.JobId)
		assert.Equal(t, "Performance bisect on win-10-perf/v8.browsing_desktop", j1.Name)
		assert.Equal(t, "v8.browsing_desktop", j1.Benchmark)
		assert.Equal(t, "win-10-perf", j1.Configuration)
		assert.Equal(t, "browse:news:reddit:2020", j1.Story)
		assert.Equal(t, pb.JobType_JOB_TYPE_BISECT, j1.JobType)
		assert.Equal(t, "maximsheshukov@google.com", j1.User)
		assert.Equal(t, pb.JobStatus_JOB_STATUS_COMPLETED, j1.JobStatus)

		expectedTime1, err := time.Parse(legacyCreatedTimeLayout, "2026-04-22T13:24:39.368547")
		require.NoError(t, err)
		assert.Equal(t, timestamppb.New(expectedTime1).AsTime(), j1.Created.AsTime())
	})

	t.Run("Returns parsed try job on success", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/jobs", r.URL.Path)
			assert.Equal(t, "user=test-user@google.com", r.URL.Query().Get("filter"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
			  "jobs": [
			    {
			      "job_id": "161bd991890000",
			      "configuration": "linux-perf",
			      "results_url": "/api/results2-serve/161bd991890000",
			      "improvement_direction": 1,
			      "arguments": {
				"comparison_mode": "try",
				"target": "performance_test_suite",
				"base_git_hash": "HEAD",
				"end_git_hash": "HEAD",
				"trace": "mail-client-read.html",
				"tags": "{\"test_path\": \"ChromiumPerf/...\"}",
				"try": "on",
				"initial_attempt_count": "50",
				"configuration": "linux-perf",
				"benchmark": "blink_perf.owp_storage",
				"story": "",
				"story_tags": "all",
				"chart": "",
				"statistic": "",
				"comparison_magnitude": "0.7795000000000005",
				"extra_test_args": "",
				"commit": "on,on",
				"base_patch": "",
				"experiment_patch": "",
				"base_extra_args": "",
				"experiment_extra_args": "",
				"project": "chromium",
				"bug_id": "496947065",
				"batch_id": ""
			      },
			      "bug_id": 496947065,
			      "project": "chromium",
			      "comparison_mode": "try",
			      "name": "Try job on linux-perf/blink_perf.owp_storage",
			      "user": "maximsheshukov@google.com",
			      "created": "2026-04-21T11:36:08.294707",
			      "updated": "2026-04-21T13:06:23.766768",
			      "started_time": "2026-04-21T11:37:04.296004",
			      "difference_count": null,
			      "exception": null,
			      "status": "Cancelled",
			      "cancel_reason": "maximsheshukov@google.com: 123",
			      "batch_id": "e34d8438-fa2e-4746-a0ac-b82d98c67da2",
			      "bots": [
				"lin-15-g582"
			      ]
			    }
			  ],
			  "count": 10,
			  "max_count": 1000,
			  "prev_cursor": "",
			  "next_cursor": "CjwKFAoHY3JlYXRlZBIJCNzq3t68_JMDEiBqDHN-Y2hyb21lcGVyZnIQCxIDSm9iGICApIz3pLEIDBgAIAE=",
			  "prev": false,
			  "next": true
			}`))
		})

		resp, err := pc.QueryJobList(context.Background(), &pb.QueryJobListRequest{
			User: "test-user@google.com",
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "", resp.Pagination.PrevCursor)
		assert.Equal(
			t,
			"CjwKFAoHY3JlYXRlZBIJCNzq3t68_JMDEiBqDHN-Y2hyb21lcGVyZnIQCxIDSm9iGICApIz3pLEIDBgAIAE=",
			resp.Pagination.NextCursor,
		)
		assert.False(t, *resp.Pagination.HasPrev)
		assert.True(t, *resp.Pagination.HasNext)
		assert.Len(t, resp.Jobs, 1)

		j2 := resp.Jobs[0]
		assert.Equal(t, "161bd991890000", j2.JobId)
		assert.Equal(t, "Try job on linux-perf/blink_perf.owp_storage", j2.Name)
		assert.Equal(t, "blink_perf.owp_storage", j2.Benchmark)
		assert.Equal(t, "linux-perf", j2.Configuration)
		assert.Equal(t, "", j2.Story)
		assert.Equal(t, pb.JobType_JOB_TYPE_TRY, j2.JobType)
		assert.Equal(t, "maximsheshukov@google.com", j2.User)
		assert.Equal(t, pb.JobStatus_JOB_STATUS_CANCELLED, j2.JobStatus)

		expectedTime2, err := time.Parse(legacyCreatedTimeLayout, "2026-04-21T11:36:08.294707")
		require.NoError(t, err)
		assert.Equal(t, timestamppb.New(expectedTime2).AsTime(), j2.Created.AsTime())
	})

	t.Run("Returns error if non-200 status code", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "Internal Server Error"}`))
		})

		resp, err := pc.QueryJobList(context.Background(), &pb.QueryJobListRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Internal Server Error")
		assert.Nil(t, resp)
	})
}

func TestCombineExtraBrowserArgs(t *testing.T) {
	testCases := []struct {
		name             string
		benchmark        string
		expected         string
		extraBrowserArgs []string
	}{
		{
			name:             "empty args non-crossbench",
			extraBrowserArgs: []string{},
			benchmark:        "testBenchmark",
			expected:         "",
		},
		{
			name:             "empty args crossbench",
			extraBrowserArgs: []string{},
			benchmark:        "testBenchmark.crossbench",
			expected:         "",
		},
		{
			name:             "non-empty args non-crossbench",
			extraBrowserArgs: []string{"--flag-a", "--flag-b"},
			benchmark:        "testBenchmark",
			expected:         `--extra-browser-args="--flag-a --flag-b"`,
		},
		{
			name:             "non-empty args crossbench",
			extraBrowserArgs: []string{"--flag-a", "--flag-b"},
			benchmark:        "testBenchmark.crossbench",
			expected:         `--flag-a --flag-b`,
		},
		{
			name:             "args with empty strings non-crossbench",
			extraBrowserArgs: []string{"", "--flag-a  ", ""},
			benchmark:        "testBenchmark",
			expected:         `--extra-browser-args="--flag-a"`,
		},
		{
			name:             "args with empty strings crossbench",
			extraBrowserArgs: []string{" ", "  --flag-a", ""},
			benchmark:        "testBenchmark.crossbench",
			expected:         `--flag-a`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := combineExtraBrowserArgs(tc.extraBrowserArgs, tc.benchmark)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestGetExtraArgsString(t *testing.T) {
	testCases := []struct {
		name      string
		extraArgs *pb.ExtraArgs
		benchmark string
		expected  string
	}{
		{
			name:      "nil extra args",
			extraArgs: nil,
			benchmark: "testBenchmark",
			expected:  "",
		},
		{
			name:      "empty extra args",
			extraArgs: &pb.ExtraArgs{},
			benchmark: "testBenchmark",
			expected:  "",
		},
		{
			name: "all browser args non-crossbench",
			extraArgs: &pb.ExtraArgs{
				ExtraBrowserArgs: "--browser-flag",
				JsFlags:          "flag-b",
				EnableFeatures:   "FeatureA",
				DisableFeatures:  "FeatureB",
			},
			benchmark: "testBenchmark",
			expected: `--extra-browser-args="--browser-flag --js-flags=flag-b ` +
				`--enable-features=FeatureA --disable-features=FeatureB"`,
		},
		{
			name: "all browser args crossbench",
			extraArgs: &pb.ExtraArgs{
				ExtraBrowserArgs: "--browser-flag",
				JsFlags:          "flag-b",
				EnableFeatures:   "FeatureA",
				DisableFeatures:  "FeatureB",
			},
			benchmark: "testBenchmark.crossbench",
			expected: "--browser-flag --js-flags=flag-b --enable-features=FeatureA " +
				"--disable-features=FeatureB",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := getExtraArgsString(tc.extraArgs, tc.benchmark)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestBuildCreateTryJobRequestURL_ValidationErrors(t *testing.T) {
	validReq := func() *pb.CreateTryJobRequest {
		return &pb.CreateTryJobRequest{
			Benchmark:     "testBenchmark",
			Configuration: "testConfig",
			Story:         "testStory",
			AttemptCount:  30,
			Base: &pb.VariantConfig{
				Commit: "baseCommit",
			},
			Experiment: &pb.VariantConfig{
				Commit: "expCommit",
			},
			User: "test-user@google.com",
		}
	}

	testCases := []struct {
		name        string
		modify      func(*pb.CreateTryJobRequest)
		expectedErr string
	}{
		{
			name: "missing benchmark",
			modify: func(r *pb.CreateTryJobRequest) {
				r.Benchmark = ""
			},
			expectedErr: "Benchmark must be specified",
		},
		{
			name: "missing configuration",
			modify: func(r *pb.CreateTryJobRequest) {
				r.Configuration = ""
			},
			expectedErr: "Configuration must be specified",
		},

		{
			name: "attempt count zero",
			modify: func(r *pb.CreateTryJobRequest) {
				r.AttemptCount = 0
			},
			expectedErr: "Attempt count should be greater than zero",
		},
		{
			name: "attempt count negative",
			modify: func(r *pb.CreateTryJobRequest) {
				r.AttemptCount = -5
			},
			expectedErr: "Attempt count should be greater than zero",
		},
		{
			name: "missing base variant",
			modify: func(r *pb.CreateTryJobRequest) {
				r.Base = nil
			},
			expectedErr: "Base variant configuration is required",
		},
		{
			name: "missing experiment variant",
			modify: func(r *pb.CreateTryJobRequest) {
				r.Experiment = nil
			},
			expectedErr: "Experiment variant configuration is required",
		},
		{
			name: "empty user",
			modify: func(r *pb.CreateTryJobRequest) {
				r.User = ""
			},
			expectedErr: "User email must be specified",
		},
		{
			name: "invalid bug id",
			modify: func(r *pb.CreateTryJobRequest) {
				bugId := int64(0)
				r.BugId = &bugId
			},
			expectedErr: "Bug ID should be greater than zero",
		},
		{
			name: "negative bug id",
			modify: func(r *pb.CreateTryJobRequest) {
				bugId := int64(-10)
				r.BugId = &bugId
			},
			expectedErr: "Bug ID should be greater than zero",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := validReq()
			tc.modify(r)
			_, err := buildCreateTryJobRequestURL(r)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestBuildCreateTryJobRequestURL_Success(t *testing.T) {
	t.Run("all custom parameters", func(t *testing.T) {
		bugId := int64(9876)
		req := &pb.CreateTryJobRequest{
			Benchmark:     "testBenchmark",
			Configuration: "testConfig",
			Story:         "testStory",
			StoryTags:     "tag1,tag2",
			AttemptCount:  45,
			Base: &pb.VariantConfig{
				Commit: "baseCommit",
				Patch:  "basePatch",
				ExtraArgs: &pb.ExtraArgs{
					BenchmarkRunnerArgs: "--runner-flag-a",
					ExtraBrowserArgs:    "--browser-flag-a",
					JsFlags:             "flag-a",
					EnableFeatures:      "FeatureA",
					DisableFeatures:     "FeatureB",
				},
			},
			Experiment: &pb.VariantConfig{
				Commit: "expCommit",
				Patch:  "expPatch",
				ExtraArgs: &pb.ExtraArgs{
					BenchmarkRunnerArgs: "--runner-flag-b",
					ExtraBrowserArgs:    "--browser-flag-b",
					JsFlags:             "flag-b",
					EnableFeatures:      "FeatureC",
					DisableFeatures:     "FeatureD",
				},
			},
			User:    "test-user@google.com",
			JobName: "Custom Try Job",
			BugId:   &bugId,
		}

		urlStr, err := buildCreateTryJobRequestURL(req)
		require.NoError(t, err)

		parsedURL, err := url.Parse(urlStr)
		require.NoError(t, err)

		expected := url.Values{
			"comparison_mode":       []string{"try"},
			"benchmark":             []string{"testBenchmark"},
			"configuration":         []string{"testConfig"},
			"story":                 []string{"testStory"},
			"story_tags":            []string{"tag1,tag2"},
			"initial_attempt_count": []string{"45"},
			"base_git_hash":         []string{"baseCommit"},
			"end_git_hash":          []string{"expCommit"},
			"base_patch":            []string{"basePatch"},
			"experiment_patch":      []string{"expPatch"},
			"base_extra_args": []string{
				`--runner-flag-a --extra-browser-args="--browser-flag-a --js-flags=` +
					`flag-a --enable-features=FeatureA --disable-features=FeatureB"`,
			},
			"experiment_extra_args": []string{
				`--runner-flag-b --extra-browser-args="--browser-flag-b --js-flags=` +
					`flag-b --enable-features=FeatureC --disable-features=FeatureD"`,
			},
			"user":   []string{"test-user@google.com"},
			"name":   []string{"Custom Try Job"},
			"bug_id": []string{"9876"},
			"tags":   []string{`{"origin":"New Pinpoint"}`},
		}

		assert.Equal(t, expected, parsedURL.Query())
	})

	t.Run("default job name", func(t *testing.T) {
		req := &pb.CreateTryJobRequest{
			Benchmark:     "testBenchmark",
			Configuration: "testConfig",
			Story:         "testStory",
			AttemptCount:  30,
			Base: &pb.VariantConfig{
				Commit: "baseCommit",
			},
			Experiment: &pb.VariantConfig{
				Commit: "expCommit",
			},
			User: "test-user@google.com",
		}

		urlStr, err := buildCreateTryJobRequestURL(req)
		require.NoError(t, err)

		parsedURL, err := url.Parse(urlStr)
		require.NoError(t, err)
		assert.Equal(t, "Try job on testConfig/testBenchmark", parsedURL.Query().Get("name"))
	})

	t.Run("empty story", func(t *testing.T) {
		req := &pb.CreateTryJobRequest{
			Benchmark:     "testBenchmark",
			Configuration: "testConfig",
			AttemptCount:  30,
			Base: &pb.VariantConfig{
				Commit: "baseCommit",
			},
			Experiment: &pb.VariantConfig{
				Commit: "expCommit",
			},
			User: "test-user@google.com",
		}

		urlStr, err := buildCreateTryJobRequestURL(req)
		require.NoError(t, err)

		parsedURL, err := url.Parse(urlStr)
		require.NoError(t, err)
		assert.False(t, parsedURL.Query().Has("story"))
	})
}

func TestCreatePinpointTryJob(t *testing.T) {
	validReq := func() *pb.CreateTryJobRequest {
		return &pb.CreateTryJobRequest{
			Benchmark:     "testBenchmark",
			Configuration: "testConfig",
			Story:         "testStory",
			AttemptCount:  30,
			Base: &pb.VariantConfig{
				Commit: "baseCommit",
			},
			Experiment: &pb.VariantConfig{
				Commit: "expCommit",
			},
			User: "somebody@google.com",
		}
	}

	t.Run("successful request creation", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/api/new", r.URL.Path)

			// verify some key parameters in post request URL
			assert.Equal(t, "testBenchmark", r.URL.Query().Get("benchmark"))
			assert.Equal(t, "testConfig", r.URL.Query().Get("configuration"))
			assert.Equal(t, "testStory", r.URL.Query().Get("story"))
			assert.Equal(t, "30", r.URL.Query().Get("initial_attempt_count"))
			assert.Equal(t, "somebody@google.com", r.URL.Query().Get("user"))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jobId": "try-job-123", "jobUrl": "https://example.com/123"}`))
		})

		pc.createPinpointTryJobCalled.Reset()
		pc.createPinpointTryJobFailed.Reset()

		resp, err := pc.CreatePinpointTryJob(context.Background(), validReq())
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "try-job-123", resp.JobId)
		assert.Equal(t, int64(1), pc.createPinpointTryJobCalled.Get())
		assert.Equal(t, int64(0), pc.createPinpointTryJobFailed.Get())
	})

	t.Run("validation failure returns error before sending HTTP request", func(t *testing.T) {
		// Mock HTTP server that should never be called
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			t.Fail()
		})

		pc.createPinpointTryJobCalled.Reset()
		pc.createPinpointTryJobFailed.Reset()

		invalidReq := validReq()
		invalidReq.Benchmark = "" // trigger validation error

		resp, err := pc.CreatePinpointTryJob(context.Background(), invalidReq)
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Failed to generate Pinpoint request URL.")
		assert.Contains(t, err.Error(), "Benchmark must be specified")
		assert.Equal(t, int64(1), pc.createPinpointTryJobCalled.Get())
		assert.Equal(t, int64(1), pc.createPinpointTryJobFailed.Get())
	})

	t.Run("http status error returns error", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "something went wrong on server"}`))
		})

		pc.createPinpointTryJobCalled.Reset()
		pc.createPinpointTryJobFailed.Reset()

		resp, err := pc.CreatePinpointTryJob(context.Background(), validReq())
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "[Error 500] something went wrong on server")
		assert.Equal(t, int64(1), pc.createPinpointTryJobCalled.Get())
		assert.Equal(t, int64(1), pc.createPinpointTryJobFailed.Get())
	})

	t.Run("invalid json response from server returns error", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`invalid-json`))
		})

		pc.createPinpointTryJobCalled.Reset()
		pc.createPinpointTryJobFailed.Reset()

		resp, err := pc.CreatePinpointTryJob(context.Background(), validReq())
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Failed to parse pinpoint response body.")
		assert.Equal(t, int64(1), pc.createPinpointTryJobCalled.Get())
		assert.Equal(t, int64(1), pc.createPinpointTryJobFailed.Get())
	})
}
