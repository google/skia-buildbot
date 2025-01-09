package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
)

func TestAnomaliesApi_CleanTestName_Default(t *testing.T) {
	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	err := json.Unmarshal(configFileBytes, &config.Config)
	config.Config.InvalidParamCharRegex = ""
	require.NoError(t, err)

	// ':': allowed in config, not in default
	// '-': allowed in both.
	// '?': now allowed in both.
	testName := "master/bot/measurement/test/sub:test?1-name"
	cleanedName, err := cleanTestName(testName)

	require.Equal(t, "master/bot/measurement/test/sub_test_1-name", cleanedName)
}

func TestAnomaliesApi_CleanTestName_FromConfig(t *testing.T) {
	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	err := json.Unmarshal(configFileBytes, &config.Config)
	require.NoError(t, err)

	testName := "master/bot/measurement/test/sub:test?1-name"
	cleanedName, err := cleanTestName(testName)

	require.Equal(t, "master/bot/measurement/test/sub:test_1-name", cleanedName)
}
