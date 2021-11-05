package ios

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machine"
)

func TestBatteryLevel_CommandSucceeds_Success(t *testing.T) {
	actual, err := fakeBatteryLevelResults(t, "Test_FakeExe_IDeviceInfo_PrintsInt")
	assert.Equal(t, 33, actual)
	assert.NoError(t, err)
}

func TestBatteryLevel_CommandEmitsNonInt_BadBatteryLevelAndError(t *testing.T) {
	actual, err := fakeBatteryLevelResults(t, "Test_FakeExe_IDeviceInfo_PrintsStringButReturnsSuccess")
	assert.Equal(t, machine.BadBatteryLevel, actual)
	assert.Error(t, err)
}

// Even if the output looks valid, consider it bad if the command's exit status is bad, just to be
// safe.
func TestBatteryLevel_CommandCrashes_BadBatteryLevelAndError(t *testing.T) {
	actual, err := fakeBatteryLevelResults(t, "Test_FakeExe_IDeviceInfo_PrintsIntButReturnsError")
	assert.Equal(t, machine.BadBatteryLevel, actual)
	assert.Error(t, err)
}

// Return the results of an ios.BatteryLevel() call, delegating to a given mocked-out
// battery-level-checking command.
func fakeBatteryLevelResults(t *testing.T, fakeTestName string) (int, error) {
	unittest.MediumTest(t) // because it shells out and thus reads from disk
	ctx := executil.FakeTestsContext(fakeTestName)
	ios := New()
	return ios.BatteryLevel(ctx)
}

func Test_FakeExe_IDeviceInfo_PrintsInt(t *testing.T) {
	fakeBatteryLevelCommand(t, "33", 0)
}

func Test_FakeExe_IDeviceInfo_PrintsIntButReturnsError(t *testing.T) {
	fakeBatteryLevelCommand(t, "44", -1)
}

func Test_FakeExe_IDeviceInfo_PrintsStringButReturnsSuccess(t *testing.T) {
	fakeBatteryLevelCommand(t, "totally not an int", 0)
}

// fakeBatteryLevelCommand pretends to be the command that checks iOS battery level, printing the
// given output and returning the given status code.
func fakeBatteryLevelCommand(t *testing.T, output string, statusCode int) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	require.Equal(t, []string{"ideviceinfo", "--domain", "com.apple.mobile.battery", "-k", "BatteryCurrentCapacity"}, executil.OriginalArgs())

	_, _ = fmt.Println(output)
	os.Exit(statusCode)
}
