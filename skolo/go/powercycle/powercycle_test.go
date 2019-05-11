package powercycle

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

var deviceIDs = []string{}

func TestConfig(t *testing.T) {
	unittest.MediumTest(t)

	dev, err := DeviceGroupFromJson5File("./example.json5", false)
	assert.NoError(t, err)
	assert.Equal(t, 25, len(dev.DeviceIDs()))

	conf, err := readConfig("./example.json5")
	assert.NoError(t, err)

	for _, oneConf := range conf.Arduino {
		assert.NotEqual(t, "", oneConf.Address)
		assert.NotEqual(t, 0, len(oneConf.DevPortMap))
	}

	for _, oneConf := range conf.EdgeSwitch {
		assert.NotEqual(t, "", oneConf.Address)
		assert.NotEqual(t, 0, len(oneConf.DevPortMap))
	}

	for _, oneConf := range conf.MPower {
		assert.NotEqual(t, "", oneConf.Address)
		assert.NotEqual(t, 0, len(oneConf.DevPortMap))
		assert.NotEqual(t, "", oneConf.User)
	}

	for _, oneConf := range conf.Seeeduino {
		assert.NotEqual(t, "", oneConf.Address)
		assert.NotEqual(t, "", oneConf.BaseURL)
		assert.NotEqual(t, 0, len(oneConf.DevPortMap))
	}
}
