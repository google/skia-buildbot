package pinpoint

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/httputils"
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

	builtURL := buildBisectJobRequestURL(req, false)
	assert.Contains(t, builtURL, chromeperfLegacyBisectURL)

	parsedURL, err := url.Parse(builtURL)
	assert.NoError(t, err)

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

	builtURL := buildBisectJobRequestURL(req, true)
	assert.Contains(t, builtURL, chromeperfLegacyBisectURL)

	parsedURL, err := url.Parse(builtURL)
	assert.NoError(t, err)

	// Alert IDs should not present.
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
	builtURL := buildBisectJobRequestURL(req, false)
	parsedURL, err := url.Parse(builtURL)
	assert.NoError(t, err)

	expected := url.Values{
		"bug_id":    []string{""},
		"test_path": []string{""},
	}
	assert.Equal(t, expected, parsedURL.Query())
}

func TestDotify(t *testing.T) {
	var tests = []struct {
		name        string
		input       string
		expectation string
	}{
		{"Empty string", "", ""},
		{"Nothing to replace", "asd", "asd"},
		{"Multiple underscores", "__qwe_asd___zxc_", "..qwe.asd...zxc."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectation, dotify(test.input))
		})
	}
}

type mockTransport struct {
	handler http.HandlerFunc
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	m.handler(recorder, req)
	return recorder.Result(), nil
}

func setupTestMocks(t *testing.T, handler http.HandlerFunc) *Client {
	client := httputils.DefaultClientConfig().WithoutRetries().Client()
	client.Transport = &mockTransport{
		handler: handler,
	}

	pc := &Client{
		httpClient: client,
	}

	return pc
}

func TestDoPostRequest(t *testing.T) {
	t.Run("Returns parsed response on success", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/new", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jobId": "12345", "jobUrl": "https://example.com/job/12345"}`))
		})

		resp, err := pc.doPostRequest(context.Background(), pinpointLegacyURL)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "12345", resp.JobID)
		assert.Equal(t, "https://example.com/job/12345", resp.JobURL)
	})

	t.Run("Returns error if non-200 status code", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "Internal Server Error"}`))
		})

		resp, err := pc.doPostRequest(context.Background(), pinpointLegacyURL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Internal Server Error")
		assert.Nil(t, resp)
	})

	t.Run("Returns error if invalid JSON response", func(t *testing.T) {
		pc := setupTestMocks(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{invalid json}`))
		})

		resp, err := pc.doPostRequest(context.Background(), pinpointLegacyURL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to parse pinpoint response body")
		assert.Nil(t, resp)
	})
}
