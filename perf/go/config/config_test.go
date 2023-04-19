package config

import (
	"path/filepath"
	"regexp"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	instanceConfig, schemaErrors, err := InstanceConfigFromFile("./testdata/invalid_regex.json")
	require.Len(t, schemaErrors, 0)
	require.NoError(t, err)

	if instanceConfig.InvalidParamCharRegex != "" {
		_, err := regexp.Compile(instanceConfig.InvalidParamCharRegex)
		require.Error(t, err)
	}
}
