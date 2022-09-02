package powercycle

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machineserver/rpc"
)

func TestControllerFromJSON5_ConfigIsNonEmpty(t *testing.T) {

	allMachines := []DeviceID{}
	controllerInitCallback := func(update rpc.UpdatePowerCycleStateRequest) error {
		for _, m := range update.Machines {
			allMachines = append(allMachines, DeviceID(m.MachineID))
			require.Equal(t, machine.Available, m.PowerCycleState)
		}
		return nil
	}
	agg, err := ControllerFromJSON5(context.Background(), "./example.json5", false, controllerInitCallback)
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
	assert.ElementsMatch(t, allMachines, agg.DeviceIDs(), "All machines are passed to ControllerInitCB.")

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

func TestControllerFromJSON5_ControllerInitCBReturnsError_ControllerFromJSON5ReturnsError(t *testing.T) {

	controllerInitCallback := func(update rpc.UpdatePowerCycleStateRequest) error {
		return fmt.Errorf("my fake error")
	}
	_, err := ControllerFromJSON5(context.Background(), "./example.json5", false, controllerInitCallback)
	require.Error(t, err)
}

func TestUpdatePowerCycleStateRequestFromController_ControllerIsNil_StillReturnsValidUpdatePowerCycleStateRequest(t *testing.T) {
	actual := updatePowerCycleStateRequestFromController(nil, machine.Available)
	require.NotNil(t, actual)
}
