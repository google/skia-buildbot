package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTraceURL(t *testing.T) {

	tests := []struct {
		traceURL       string
		expectedBucket string
		expectedPath   string
		expectedError  bool
		name           string
	}{
		{
			traceURL:       "https://storage.cloud.google.com/chrome-telemetry-output/xyz/retry_0/trace.html",
			expectedBucket: "chrome-telemetry-output",
			expectedPath:   "xyz/retry_0/trace.html",
			expectedError:  false,
			name:           "Well-formed URL with bucket and path",
		},
		{
			traceURL:       "https://console.developers.google.com/m/cloudstorage/b/chrome-telemetry-output/xyz/retry_0/trace.html",
			expectedBucket: "",
			expectedPath:   "",
			expectedError:  true,
			name:           "Malformed URL",
		},
	}

	for _, test := range tests {
		bucket, path, err := parseTraceURL(test.traceURL)
		require.Equal(t, test.expectedBucket, bucket, test.name)
		require.Equal(t, test.expectedPath, path, test.name)
		if test.expectedError {
			require.Error(t, err, test.name)
		} else {
			require.NoError(t, err, test.name)
		}
	}
}
