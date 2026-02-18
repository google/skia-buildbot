package pinpoint

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestBuildBisectRequestUrlPopulatesAllFields(t *testing.T) {
	req := CreateBisectRequest{
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

	builtURL := buildBisectRequestURL(req)
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
		"tags":                 []string{`{"origin":"skia_perf"}`},
	}
	assert.Equal(t, expected, parsedURL.Query())
}

func TestBuildBisectRequestUrlPopulatesRequiredFields(t *testing.T) {
	req := CreateBisectRequest{}
	builtURL := buildBisectRequestURL(req)
	parsedURL, err := url.Parse(builtURL)
	assert.NoError(t, err)

	expected := url.Values{
		"bug_id":    []string{""},
		"test_path": []string{""},
		"tags":      []string{`{"origin":"skia_perf"}`},
	}
	assert.Equal(t, expected, parsedURL.Query())
}
