package footers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/deepequal"
)

func TestParseIncludeTryjobsFooter(t *testing.T) {
	tests := []struct {
		footer         string
		expectedOutput map[string][]string
		expectedError  bool
	}{
		{
			footer: "bucket1:bot1,bot2;bucket2:bot3,bot4",
			expectedOutput: map[string][]string{
				"bucket1": []string{"bot1", "bot2"},
				"bucket2": []string{"bot3", "bot4"},
			},
			expectedError: false,
		},
		{
			footer: "bucket1:bot1,bot2,bot3",
			expectedOutput: map[string][]string{
				"bucket1": []string{"bot1", "bot2", "bot3"},
			},
			expectedError: false,
		},
		{
			footer:         "",
			expectedOutput: nil,
			expectedError:  true,
		},
		{
			footer:         "bucket1:bot1:bot2",
			expectedOutput: nil,
			expectedError:  true,
		},
		{
			footer:         "bucket1:bot1,bot2;bucket2",
			expectedOutput: nil,
			expectedError:  true,
		},
	}

	for _, test := range tests {
		output, err := ParseIncludeTryjobsFooter(test.footer)
		if test.expectedError {
			require.Error(t, err)
		} else {
			require.Nil(t, err)
		}
		require.True(t, deepequal.DeepEqual(test.expectedOutput, output))
	}
}

func TestGetFootersMap(t *testing.T) {
	tests := []struct {
		commitMsg      string
		expectedOutput map[string]string
	}{
		{
			commitMsg:      "Test test test\n\nfooter: value",
			expectedOutput: map[string]string{"footer": "value"},
		},
		{
			commitMsg:      "Test test test\n\nfooter-no-space:value",
			expectedOutput: map[string]string{"footer-no-space": "value"},
		},
		{
			commitMsg:      "Test test test\nfake-footer: value",
			expectedOutput: map[string]string{},
		},
		{
			commitMsg:      "Test test test\nfake-footer: value\n\nfooter1: value1\nfooter2: value2",
			expectedOutput: map[string]string{"footer1": "value1", "footer2": "value2"},
		},
	}

	for _, test := range tests {
		require.True(t, deepequal.DeepEqual(test.expectedOutput, GetFootersMap(test.commitMsg)))
	}
}

func TestGetBoolVal(t *testing.T) {
	tests := []struct {
		footersMap      map[string]string
		supportedFooter CQSupportedFooter
		expectedOutput  bool
	}{
		{
			footersMap: map[string]string{
				string(RerunTryjobsFooter): "true",
			},
			supportedFooter: RerunTryjobsFooter,
			expectedOutput:  true,
		},
		{
			footersMap: map[string]string{
				string(RerunTryjobsFooter): "false",
			},
			supportedFooter: RerunTryjobsFooter,
			expectedOutput:  false,
		},
		{
			footersMap: map[string]string{
				"some-other-footer": "true",
			},
			supportedFooter: RerunTryjobsFooter,
			expectedOutput:  false,
		},
		{
			footersMap: map[string]string{
				"some-other-footer":        "true",
				string(RerunTryjobsFooter): "true",
			},
			supportedFooter: RerunTryjobsFooter,
			expectedOutput:  true,
		},
		{
			footersMap:      map[string]string{},
			supportedFooter: RerunTryjobsFooter,
			expectedOutput:  false,
		},
		{
			footersMap:      nil,
			supportedFooter: RerunTryjobsFooter,
			expectedOutput:  false,
		},
		{
			footersMap: map[string]string{
				string(RerunTryjobsFooter): "not-a-bool-val",
			},
			supportedFooter: RerunTryjobsFooter,
			expectedOutput:  false,
		},
	}

	for _, test := range tests {
		require.Equal(t, test.expectedOutput, GetBoolVal(test.footersMap, test.supportedFooter, 1))
	}
}

func TestGetStringVal(t *testing.T) {
	tests := []struct {
		footersMap      map[string]string
		supportedFooter CQSupportedFooter
		expectedOutput  string
	}{
		{
			footersMap: map[string]string{
				string(IncludeTryjobsFooter): "value",
			},
			supportedFooter: IncludeTryjobsFooter,
			expectedOutput:  "value",
		},
		{
			footersMap: map[string]string{
				"some-other-footer": "value",
			},
			supportedFooter: IncludeTryjobsFooter,
			expectedOutput:  "",
		},
		{
			footersMap: map[string]string{
				"some-other-footer":          "value1",
				string(IncludeTryjobsFooter): "value2",
			},
			supportedFooter: IncludeTryjobsFooter,
			expectedOutput:  "value2",
		},
		{
			footersMap:      map[string]string{},
			supportedFooter: IncludeTryjobsFooter,
			expectedOutput:  "",
		},
		{
			footersMap:      nil,
			supportedFooter: IncludeTryjobsFooter,
			expectedOutput:  "",
		},
	}

	for _, test := range tests {
		require.Equal(t, test.expectedOutput, GetStringVal(test.footersMap, test.supportedFooter))
	}
}
