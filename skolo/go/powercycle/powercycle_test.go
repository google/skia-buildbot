package powercycle

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestConfig(t *testing.T) {
	unittest.MediumTest(t)

	dev, err := DeviceGroupFromJson5File("./example.json5", false)
	require.NoError(t, err)
	require.Equal(t, 25, len(dev.DeviceIDs()))

	conf, err := readConfig("./example.json5")
	require.NoError(t, err)

	for _, oneConf := range conf.EdgeSwitch {
		require.NotEqual(t, "", oneConf.Address)
		require.NotEqual(t, 0, len(oneConf.DevPortMap))
	}

	for _, oneConf := range conf.MPower {
		require.NotEqual(t, "", oneConf.Address)
		require.NotEqual(t, 0, len(oneConf.DevPortMap))
		require.NotEqual(t, "", oneConf.User)
	}
}
