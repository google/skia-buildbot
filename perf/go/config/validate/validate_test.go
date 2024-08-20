package validate

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/notifytypes"
	"go.skia.org/infra/perf/go/types"
)

func TestInstanceConfigBytes_AllExistingConfigs_ShouldBeValid(t *testing.T) {
	allExistingConfigs, err := filepath.Glob(filepath.Join("..", "..", "..", "configs", "*.json"))
	require.Greater(t, len(allExistingConfigs), 0)
	require.NoError(t, err)
	for _, filename := range allExistingConfigs {
		instanceConfig, schemaErrors, err := InstanceConfigFromFile(filename)
		require.Empty(t, schemaErrors, filename)
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
	_, _, err := InstanceConfigFromFile(testutils.TestDataFilename(t, "empty.json"))
	require.Error(t, err)
}

func TestInstanceConfig_WithInvalidRegex_CauseError(t *testing.T) {
	_, schemaErrors, err := InstanceConfigFromFile(testutils.TestDataFilename(t, "invalid_regex.json"))
	require.Empty(t, schemaErrors)
	require.Contains(t, err.Error(), "compiling invalid_param_char_regex")
}

func TestInstanceConfigBytes_ContainsValidNotificationTemplates_ShouldBeValid(t *testing.T) {
	_, _, err := InstanceConfigFromFile(testutils.TestDataFilename(t, "valid-notify-template.json"))
	require.NoError(t, err)
}

func TestInstanceConfigBytes_ContainsInValidNotificationTemplates_ShouldBeInValid(t *testing.T) {
	_, _, err := InstanceConfigFromFile(testutils.TestDataFilename(t, "invalid-notify-template.json"))
	require.Contains(t, err.Error(), "can't evaluate field FOO")
}

func TestInstanceConfigValidate_MarkdownIssueTrackerButAPIKeySecretProjectNotSet_ReturnsError(t *testing.T) {
	i := config.InstanceConfig{
		NotifyConfig: config.NotifyConfig{
			Notifications: notifytypes.MarkdownIssueTracker,
		},
	}
	require.Contains(t, Validate(i).Error(), "issue_tracker_api_key_secret_project must be supplied")
}

func TestInstanceConfigValidate_MarkdownIssueTrackerButAPIKeySecretNameNotSet_ReturnsError(t *testing.T) {
	i := config.InstanceConfig{
		NotifyConfig: config.NotifyConfig{
			Notifications:                   notifytypes.MarkdownIssueTracker,
			IssueTrackerAPIKeySecretProject: "skia-public",
		},
	}
	require.Contains(t, Validate(i).Error(), "issue_tracker_api_key_secret_name must be supplied")
}

func TestInstanceConfigValidate_CulpritNotify_MarkdownIssueTrackerButAPIKeySecretProjectNotSet_ReturnsError(t *testing.T) {
	i := config.InstanceConfig{
		CulpritNotifyConfig: config.CulpritNotifyConfig{
			NotificationType: types.IssueNotify,
		},
	}
	require.Contains(t, Validate(i).Error(), "issue_tracker_api_key_secret_project must be supplied")
}

func TestInstanceConfigValidate_CulpritNotify_MarkdownIssueTrackerButAPIKeySecretNameNotSet_ReturnsError(t *testing.T) {
	i := config.InstanceConfig{
		CulpritNotifyConfig: config.CulpritNotifyConfig{
			NotificationType:                types.IssueNotify,
			IssueTrackerAPIKeySecretProject: "skia-public",
		},
	}
	require.Contains(t, Validate(i).Error(), "issue_tracker_api_key_secret_name must be supplied")
}

func TestInstanceConfigValidate_InvalidParamCharRegexMatchesComma_ReturnsError(t *testing.T) {
	i := config.InstanceConfig{
		InvalidParamCharRegex: ",",
	}
	require.Contains(t, Validate(i).Error(), "invalid_param_char_regex must match")
}

func TestInstanceConfigValidate_InvalidParamCharRegexMatchesEqual_ReturnsError(t *testing.T) {
	i := config.InstanceConfig{
		InvalidParamCharRegex: "=",
	}
	require.Contains(t, Validate(i).Error(), "invalid_param_char_regex must match")
}
