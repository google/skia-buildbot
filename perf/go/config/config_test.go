package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestInstanceConfigFromFile_Success(t *testing.T) {
	unittest.SmallTest(t)
	instanceConfig, err := InstanceConfigFromFile("./testdata/good_instance_config.json")
	require.NoError(t, err)
	assert.Equal(t, int32(256), instanceConfig.DataStoreConfig.TileSize)
}

func TestInstanceConfigFromFile_FailureOnMalformedJSON(t *testing.T) {
	unittest.SmallTest(t)
	_, err := InstanceConfigFromFile("./testdata/malformed_instance_config.json")
	require.Error(t, err)
}
