// check_workers_health is an application that checks the health of all CT
// workers and reports results to the admins if any worker/device is down.
package main

import (
	"fmt"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
)

func main() {
	defer common.LogPanic()
	common.Init()
	defer glog.Flush()

	// Collect unhealthy machines
	offlineMachines := []string{}
	offlineDevices := []string{}
	missingDevices := []string{}
	nonResponsiveDevices := []string{}
	// Also collect healthy machines for additional checks.
	healthyMachines := []string{}

	deviceOfflineOutputs, err := util.SSH("adb devices", util.Slaves, util.ADB_DEVICES_TIMEOUT)
	if err != nil {
		glog.Fatalf("Error while sshing into workers: %s", err)
		return
	}
	// Populate offlineMachines, offlineDevices and missingDevices.
	for hostname, out := range deviceOfflineOutputs {
		if out == "" {
			offlineMachines = append(offlineMachines, hostname)
			glog.Warningf("%s is offline", hostname)
		} else if strings.Contains(out, "offline") {
			// The adb output text contains offline devices.
			offlineDevices = append(offlineDevices, hostname)
			glog.Warningf("%s has an offline device", hostname)
		} else if strings.Count(out, "device") == 1 {
			// The adb output text only contains "List of devices attached"
			// without any devices listed below it.
			missingDevices = append(missingDevices, hostname)
			glog.Warningf("%s has missing devices", hostname)
		} else {
			// Everything seems fine so far, add this machine as a healthyMachine.
			healthyMachines = append(healthyMachines, hostname)
		}
	}

	// Populate nonResponsiveDevices.
	responsivenessOutputs, err :=
		util.SSH("adb shell uptime", healthyMachines, util.ADB_SHELL_UPTIME_TIMEOUT)
	if err != nil {
		glog.Fatalf("Error while sshing into workers: %s", err)
		return
	}
	// Clear and repopulate the healthy machines slice.
	healthyMachines = nil
	for hostname, out := range responsivenessOutputs {
		if out == "" {
			nonResponsiveDevices = append(nonResponsiveDevices, hostname)
			glog.Warningf("%s has non-responsive devices.")
		} else {
			// Everything seems fine so far, add this machine as a healthyMachine.
			healthyMachines = append(healthyMachines, hostname)
		}
	}

	// Email admins if there are any unhealthy machines.
	if len(offlineMachines) != 0 || len(offlineDevices) != 0 || len(missingDevices) != 0 || len(nonResponsiveDevices) != 0 {
		emailSubject := "There are unhealthy Cluster telemetry machines"
		emailBody := "Please file a ticket to chrome-golo-tech-ticket@ (for offline devices) and chrome-labs-tech-ticket@ (for offline machines) using https://docs.google.com/spreadsheets/d/1whlE4nDJB0XFBemJliupOORepdXf_vXyAfFgsprTAxY/edit#gid=0 for-<br/><br/>"
		if len(offlineMachines) != 0 {
			emailBody += fmt.Sprintf("The following machines are offline: %s<br/>", strings.Join(offlineMachines, ","))
		}
		if len(offlineDevices) != 0 {
			emailBody += fmt.Sprintf("The following machines have offline devices: %s<br/>", strings.Join(offlineDevices, ","))
		}
		if len(missingDevices) != 0 {
			emailBody += fmt.Sprintf("The following machines have missing devices: %s<br/>", strings.Join(missingDevices, ","))
		}
		if len(nonResponsiveDevices) != 0 {
			emailBody += fmt.Sprintf("The following machines have non-responsive devices: %s<br/>", strings.Join(nonResponsiveDevices, ","))
		}
		if err := util.SendEmail(util.CtAdmins, emailSubject, emailBody); err != nil {
			glog.Errorf("Error while sending email: %s", err)
			return
		}
	} else {
		glog.Info("All CT machines are healthy")
	}
}
