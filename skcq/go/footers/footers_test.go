package footers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestParseIncludeTryjobsFooter(t *testing.T) {
	unittest.SmallTest(t)

	tests := []struct {
		footer         string
		expectedOutput map[string][]string
		expectedError  bool
	}{
		{
			footer: "bucket1:bot1,bot2;bucket2:bot3,bot4",
			expectedOutput: map[string][]string{
				"bucket1": {"bot1", "bot2"},
				"bucket2": {"bot3", "bot4"},
			},
			expectedError: false,
		},
		{
			footer: "bucket1:bot1,bot2,bot3",
			expectedOutput: map[string][]string{
				"bucket1": {"bot1", "bot2", "bot3"},
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
