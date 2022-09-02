package powercycle

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPassword_EmptyIfNotSet(t *testing.T) {

	e := EdgeSwitchConfig{}
	assert.Equal(t, "", e.getPassword())
}

func TestGetPassword_SuccessIfSetInStruct(t *testing.T) {

	e := EdgeSwitchConfig{
		Password: "foo",
	}
	assert.Equal(t, "foo", e.getPassword())
}

func TestGetPassword_SuccessIfSetByEnvVar(t *testing.T) {

	e := EdgeSwitchConfig{}
	err := os.Setenv(powerCyclePasswordEnvVar, "bar")
	require.NoError(t, err)
	defer func() {
		err := os.Unsetenv(powerCyclePasswordEnvVar)
		require.NoError(t, err)
	}()
	assert.Equal(t, "bar", e.getPassword())
}

func TestNewEdgeSwitch_NewFails_ControllerIsStillReturnedAndCanListMachines(t *testing.T) {

	// Hand in a cancelled context so the attempt to talk to the mPower device
	// fails immeditely, otherwise this test takes 3s to fail.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	es, err := newEdgeSwitchController(ctx, esConfig(), true)
	require.Error(t, err)
	require.NotNil(t, es)
	require.Len(t, es.DeviceIDs(), 2)
}

func esConfig() *EdgeSwitchConfig {
	return &EdgeSwitchConfig{
		Address:  "192.168.1.117",
		User:     "ubnt",
		Password: "not-a-real-password",
		DevPortMap: map[DeviceID]int{
			"skia-rpi-001-device": 7,
			"skia-rpi-002-device": 8,
		},
	}
}
