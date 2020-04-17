package powercycle

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestControllerFromJSON5_ConfigIsNonEmpty(t *testing.T) {
	unittest.MediumTest(t)

	agg, err := ControllerFromJSON5(context.Background(), "./example.json5", false)
	require.NoError(t, err)
	assert.ElementsMatch(t, []DeviceID{
		"skia-e-linux-001",
		"skia-e-linux-002",
		"skia-e-linux-003",
		"skia-e-linux-004",
		"skia-e-linux-010",
		"skia-e-linux-011",
		"skia-e-linux-012",
		"skia-e-linux-013",
		"test-relay-1",
		"skia-rpi-003-device",
		"skia-i-rpi-096",
		"skia-i-rpi-097",
		"skia-i-rpi-098",
		"skia-i-rpi-099",
		"skia-i-rpi-196",
		"skia-i-rpi-197",
		"skia-i-rpi-198",
		"skia-i-rpi-199",
		"skia-i-rpi-296",
		"skia-i-rpi-297",
		"skia-i-rpi-298",
		"skia-i-rpi-299",
		"skia-rpi-1-TEST",
		"skia-rpi-2-TEST",
		"skia-rpi-TEST",
	}, agg.DeviceIDs())

	conf, err := readConfig("./example.json5")
	require.NoError(t, err)

	for _, oneConf := range conf.EdgeSwitch {
		require.NotEqual(t, "", oneConf.Address)
		require.NotEmpty(t, oneConf.DevPortMap)
	}

	for _, oneConf := range conf.MPower {
		require.NotEqual(t, "", oneConf.Address)
		require.NotEqual(t, "", oneConf.User)
		require.NotEmpty(t, oneConf.DevPortMap)
	}
}
