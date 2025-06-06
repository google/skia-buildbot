package pinpoint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMinimumViableSetOfRequiredFields_OK(t *testing.T) {
	tools := GetTools()
	// 2 when bisect is enabled
	require.Equal(t, 1, len(tools))
}

// Test to explicitly ensure that the required set does not change
func TestRequiredArguments_OK(t *testing.T) {
	args := arguments()
	require.Equal(t, 7, len(args))

	for _, arg := range args {
		switch arg.Name {
		case BaseGitHashFlagName:
		case BenchmarkFlagName:
		case StoryFlagName:
		case BotConfigurationFlagName:
		case ExperimentGitHashFlagName:
			require.True(t, arg.Required)
		}
	}
}
