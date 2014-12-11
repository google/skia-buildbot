// Utility that contains utility methods for interacting with adb.
package adb

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/golang/glog"

	"skia.googlesource.com/buildbot.git/ct/go/util"
)

// VerifyLocalDevice does not throw an error if an Android device is connected and
// online. An error is returned if either "adb" is not installed or if the Android
// device is offline or missing.
func VerifyLocalDevice() error {
	// Run "adb version".
	// Command should return without an error.
	if err := util.ExecuteCmd(util.BINARY_ADB, []string{"version"}, []string{}, 5*time.Minute, nil, nil); err != nil {
		return fmt.Errorf("adb not installed or not found: %s", err)
	}

	// Run "adb devices | grep offline".
	// Command should return with an error.
	devicesCmd := exec.Command(util.BINARY_ADB, "devices")
	offlineCmd := exec.Command("grep", "offline")
	offlineCmd.Stdin, _ = devicesCmd.StdoutPipe()
	offlineCmd.Stdout = util.WriteLog{LogFunc: glog.Infof, OutputFile: nil}
	offlineCmd.Start()
	devicesCmd.Run()
	if err := offlineCmd.Wait(); err == nil {
		// A nil error here means that an offline device was found.
		return fmt.Errorf("Android device is offline: %s", err)
	}

	// Running "adb devices | grep device$
	// Command should return without an error.
	devicesCmd = exec.Command(util.BINARY_ADB, "devices")
	missingCmd := exec.Command("grep", "device$")
	missingCmd.Stdin, _ = devicesCmd.StdoutPipe()
	missingCmd.Stdout = util.WriteLog{LogFunc: glog.Infof, OutputFile: nil}
	missingCmd.Start()
	devicesCmd.Run()
	if err := missingCmd.Wait(); err != nil {
		// An error here means that the device is missing.
		return fmt.Errorf("Android device is missing: %s", err)
	}

	return nil
}
