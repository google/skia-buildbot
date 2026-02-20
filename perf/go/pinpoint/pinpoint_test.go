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

func TestBuildChromeperfBisectRequestUrlPopulatesAllFields(t *testing.T) {
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

	builtURL := buildChromeperfBisectRequestURL(req)
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

func TestBuildChromeperfBisectRequestUrlPopulatesRequiredFields(t *testing.T) {
	req := CreateBisectRequest{}
	builtURL := buildChromeperfBisectRequestURL(req)
	parsedURL, err := url.Parse(builtURL)
	assert.NoError(t, err)

	expected := url.Values{
		"bug_id":    []string{""},
		"test_path": []string{""},
		"tags":      []string{`{"origin":"skia_perf"}`},
	}
	assert.Equal(t, expected, parsedURL.Query())
}

func TestBuildChromeperfBisectRequestUrlReplacesUnderscores(t *testing.T) {
	req := CreateBisectRequest{Story: "qwe_asd"}
	builtURL := buildChromeperfBisectRequestURL(req)
	parsedURL, err := url.Parse(builtURL)
	assert.NoError(t, err)
	assert.Equal(t, "qwe.asd", parsedURL.Query().Get("story"))
}

func TestBuildPinpointBisectRequestUrlPopulatesAllFields(t *testing.T) {
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

	builtURL := buildPinpointBisectRequestURL(req)
	assert.Contains(t, builtURL, pinpointLegacyURL)

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

func TestBuildPinpointBisectRequestUrlPopulatesRequiredFields(t *testing.T) {
	builtURL := buildPinpointBisectRequestURL(CreateBisectRequest{})
	assert.Contains(t, builtURL, pinpointLegacyURL)

	parsedURL, err := url.Parse(builtURL)
	assert.NoError(t, err)

	expected := url.Values{"tags": []string{`{"origin":"skia_perf"}`}}
	assert.Equal(t, expected, parsedURL.Query())
}

func TestBuildPinpointBisectRequestUrlReplacesUnderscores(t *testing.T) {
	req := CreateBisectRequest{Story: "qwe_asd"}
	builtURL := buildPinpointBisectRequestURL(req)
	parsedURL, err := url.Parse(builtURL)
	assert.NoError(t, err)
	assert.Equal(t, "qwe.asd", parsedURL.Query().Get("story"))
}

func TestGetBisectRequestUrlForNewAnomalies(t *testing.T) {
	url := getBisectRequestURL(CreateBisectRequest{}, true)
	assert.Contains(t, url, pinpointLegacyURL)
}

func TestGetBisectRequestUrlForOldAnomalies(t *testing.T) {
	url := getBisectRequestURL(CreateBisectRequest{}, false)
	assert.Contains(t, url, chromeperfLegacyBisectURL)
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
