package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestInstanceConfigFromFile_Success(t *testing.T) {
	unittest.SmallTest(t)
	f := filepath.Join(testutils.TestDataDir(t), "good_instance_config.json")
	instanceConfig, err := InstanceConfigFromFile(f)
	require.NoError(t, err)
	assert.Equal(t, int32(256), instanceConfig.DataStoreConfig.TileSize)
}

func TestInstanceConfigFromFile_FailureOnMalformedJSON(t *testing.T) {
	unittest.SmallTest(t)
	f := filepath.Join(testutils.TestDataDir(t), "malformed_instance_config.json")
	_, err := InstanceConfigFromFile(f)
	require.Error(t, err)
}
