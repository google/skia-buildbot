package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetOverrideNonProdHost(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://chrome-perf-autopush.corp.goog",
			expected: "https://chrome-perf.corp.goog",
		},
		{
			input:    "https://chrome-perf-lts.corp.goog",
			expected: "https://chrome-perf.corp.goog",
		},
		{
			input:    "https://v8-perf-qa.corp.goog",
			expected: "https://v8-perf.corp.goog",
		},
		{
			input:    "https://v8-perf-qa.luci.app",
			expected: "https://v8-perf.luci.app",
		},
		{
			input:    "https://v8-perf.corp.goog",
			expected: "https://v8-perf.corp.goog",
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			assert.Equal(t, test.expected, getOverrideNonProdHost(test.input))
		})
	}
}
