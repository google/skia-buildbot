package config

import (
	_ "embed"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/notifytypes"
)

func TestInstanceConfigBytes_AllExistingConfigs_ShouldBeValid(t *testing.T) {
	allExistingConfigs, err := filepath.Glob("../../configs/*.json")
	require.Greater(t, len(allExistingConfigs), 0)
	require.NoError(t, err)
	for _, filename := range allExistingConfigs {
		instanceConfig, schemaErrors, err := InstanceConfigFromFile(filename)
		require.Len(t, schemaErrors, 0)
		require.NoError(t, err, filename)

		if instanceConfig.InvalidParamCharRegex != "" {
			_, err := regexp.Compile(instanceConfig.InvalidParamCharRegex)
			require.NoError(t, err)

			assert.NotContains(t, instanceConfig.InvalidParamCharRegex, ',')
			assert.NotContains(t, instanceConfig.InvalidParamCharRegex, '=')
		}

		if instanceConfig.GitRepoConfig.CommitNumberRegex != "" {
			_, err := regexp.Compile(instanceConfig.GitRepoConfig.CommitNumberRegex)
			require.NoError(t, err)
		}
	}
}

func TestInstanceConfigBytes_EmptyJSONObject_ShouldBeInValid(t *testing.T) {
	_, _, err := InstanceConfigFromFile("./testdata/empty.json")
	require.Error(t, err)
}

func TestInstanceConfig_WithInvalidRegex_CauseError(t *testing.T) {
	_, schemaErrors, err := InstanceConfigFromFile("./testdata/invalid_regex.json")
	require.Len(t, schemaErrors, 0)
	require.Contains(t, err.Error(), "compiling invalid_param_char_regex")
}

func TestInstanceConfigValidate_MarkdownIssueTrackerButAPIKeySecretProjectNotSet_ReturnsError(t *testing.T) {
	i := InstanceConfig{
		NotifyConfig: NotifyConfig{
			Notifications: notifytypes.MarkdownIssueTracker,
		},
	}
	require.Contains(t, i.Validate().Error(), "issue_tracker_api_key_secret_project must be supplied")
}

func TestInstanceConfigValidate_MarkdownIssueTrackerButAPIKeySecretNameNotSet_ReturnsError(t *testing.T) {
	i := InstanceConfig{
		NotifyConfig: NotifyConfig{
			Notifications:                   notifytypes.MarkdownIssueTracker,
			IssueTrackerAPIKeySecretProject: "skia-public",
		},
	}
	require.Contains(t, i.Validate().Error(), "issue_tracker_api_key_secret_name must be supplied")
}

func TestInstanceConfigValidate_InvalidParamCharRegexMatchesComma_ReturnsError(t *testing.T) {
	i := InstanceConfig{
		InvalidParamCharRegex: ",",
	}
	require.Contains(t, i.Validate().Error(), "invalid_param_char_regex must match")
}

func TestInstanceConfigValidate_InvalidParamCharRegexMatchesEqual_ReturnsError(t *testing.T) {
	i := InstanceConfig{
		InvalidParamCharRegex: "=",
	}
	require.Contains(t, i.Validate().Error(), "invalid_param_char_regex must match")
}
