package pinpoint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMinimumViableSetOfRequiredFields_OK(t *testing.T) {
	tools := GetTools(nil, nil)
	// 2 - try, bisect
	require.Equal(t, 2, len(tools))
}

// Test to explicitly ensure that the required set does not change
func TestTryJobRequiredArguments_OK(t *testing.T) {
	args := arguments(true)
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

func TestBisectRequiredArguments_OK(t *testing.T) {
	args := bisectArguments()
	require.Equal(t, 9, len(args))

	for _, arg := range args {
		switch arg.Name {
		case BenchmarkFlagName:
		case StoryFlagName:
		case BotConfigurationFlagName:
			require.True(t, arg.Required)
		case BaseGitHashFlagName:
		case ExperimentGitHashFlagName:
			require.False(t, arg.Required)
		}
	}
}
