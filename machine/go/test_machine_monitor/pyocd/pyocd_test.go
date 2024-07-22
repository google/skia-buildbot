package pyocd

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
)

func TestDeviceType_CommandSucceeds_SuccessfullyDetectsRT1170(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_PyOCDList_PrintsRT1170String")
	pyocd := pyocdImpl{
		deviceType: "RT1170_EVK",
	}
	actual, err := pyocd.DeviceType(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "RT1170_EVK", actual)
}

func TestDeviceType_CommandSucceeds_SuccessfullyDetectsSTM32U5(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_PyOCDList_PrintsSTM32U5String")
	pyocd := pyocdImpl{
		deviceType: "STM32U5_EVK",
	}
	actual, err := pyocd.DeviceType(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "STM32U5_EVK", actual)
}

func TestDeviceType_CommandHasNoDevices_ReturnsError(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_PyOCDList_NoDevicesAttached")
	pyocd := pyocdImpl{
		deviceType: "STM32U5_EVK",
	}
	_, err := pyocd.DeviceType(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No device")
}

func TestDeviceType_CommandHasEmptyConfiguration_ReturnsError(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_ShouldNotBeCalled")
	pyocd := pyocdImpl{}
	_, err := pyocd.DeviceType(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Empty device")
}

func Test_FakeExe_PyOCDList_PrintsRT1170String(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Equal(t, []string{"python3", "-m", "pyocd", "list", "--color=never"},
		executil.OriginalArgs())

	_, _ = fmt.Println(`  #   Probe/Board             Unique ID                                          Target
---------------------------------------------------------------------------------------------------
  0   ARM DAPLink CMSIS-DAP   0244000026e0e5b300000000000000000000000097969905   ✔︎ mimxrt1170_cm7
      MIMXRT1170-EVK`)
	os.Exit(0)
}

func Test_FakeExe_PyOCDList_PrintsSTM32U5String(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Equal(t, []string{"python3", "-m", "pyocd", "list", "--color=never"},
		executil.OriginalArgs())

	_, _ = fmt.Println(`  #   Probe/Board   Unique ID                  Target
-------------------------------------------------------
  0   STLINK-V3     002E00183033510435393935   n/a`)
	os.Exit(0)
}

func Test_FakeExe_PyOCDList_NoDevicesAttached(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Equal(t, []string{"python3", "-m", "pyocd", "list", "--color=never"},
		executil.OriginalArgs())

	_, _ = fmt.Println(`No available debug probes are connected`)
	os.Exit(0)
}

func Test_FakeExe_ShouldNotBeCalled(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	panic("Should not call this function")
}
