package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/powercycle"
)

func main() {
	var (
		configFile = flag.String("conf", "/etc/powercycle.json5", "JSON5 file with device configuration.")
		delay      = flag.Int("delay", 0, "Any value > 0 overrides the default duration (in sec) between turning the port off and on.")
		connect    = flag.Bool("connect", false, "Connect to all the powercycle hosts and verify they are attached computing attached devices.")
		list       = flag.Bool("l", false, "List the available devices and exit.")
		autoFix    = flag.Bool("auto_fix", false, "Fetch the list of down bots/devices from power.skia.org and reboot those.")
		autoFixURL = flag.String("auto_fix_url", "https://power.skia.org/down_bots", "Url at which to grab the list of down bots/devices.")
		daemonURL  = flag.String("daemon_url", "http://localhost:9210/powercycled_bots", "The (probably localhost) URL at which the powercycle daemon is located.")
	)
	common.Init()
	args := flag.Args()
	if !*connect && !*autoFix && len(args) == 0 {
		sklog.Info("Skipping connection test. Use --connect flag to force connection testing.")
	} else {
		// Force connect to be on because we will be powercycling devices. If we try to
		// powercycle without connecting first, the DeviceGroups won't be properly initialized.
		*connect = true
	}
	devGroup, err := powercycle.ParseJSON5(*configFile, *connect)
	if err != nil {
		sklog.Fatalf("Unable to parse config file.  Got error: %s", err)
	}

	if *list {
		listDevices(devGroup, 0)
	}

	if *autoFix {
		args, err = getAutoFixCandidates(*autoFixURL)
		if err != nil {
			sklog.Fatalf("Could not fetch list of down bots/devices: %s", err)
			return
		}
		if len(args) == 0 {
			sklog.Errorf("Nothing to autofix.")
			os.Exit(0)
		}
		// Give the human user a chance to stop it safely.
		sklog.Infof("Will autofix %q in 5 seconds.", args)
		sklog.Info("Use Ctrl+C to cancel if this is unwanted/wrong.")
		time.Sleep(5 * time.Second)
	}

	// No device id given.
	if len(args) == 0 {
		sklog.Errorf("No device id given to power cycle.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Check if all of the device ids are valid.
	validDeviceIds := devGroup.DeviceIDs()
	for _, deviceID := range args {
		if !powercycle.DeviceIn(powercycle.DeviceID(deviceID), validDeviceIds) {
			sklog.Errorf("Invalid device ID.")
			listDevices(devGroup, 1)
		}
	}

	for _, deviceID := range args {
		if err := devGroup.PowerCycle(context.Background(), powercycle.DeviceID(deviceID), time.Duration(*delay)*time.Second); err != nil {
			sklog.Fatalf("Unable to power cycle device %s. Got error: %s", deviceID, err)
		}
	}
	if err := reportToDaemon(*daemonURL, args); err != nil {
		sklog.Fatalf("Could not report powercyling through daemon: %s", err)
	}
	sklog.Infof("Power cycle successful. All done.")
	sklog.Flush()
}

// listDevices prints out the devices it know about. This implies that
// the devices have been contacted and passed a ping test.
func listDevices(ctrl powercycle.Controller, exitCode int) {
	sklog.Errorf("Valid device IDs are:\n\n")
	for _, id := range ctrl.DeviceIDs() {
		sklog.Errorf("    %s\n", id)
	}
	os.Exit(exitCode)
}

func reportToDaemon(daemonURL string, bots []string) error {
	if daemonURL == "" {
		sklog.Warning("Skipping daemon reporting because --daemon_url is blank")
		return nil
	}
	c := httputils.NewTimeoutClient()
	body := bytes.Buffer{}
	toReport := struct {
		PowercycledBots []string `json:"powercycled_bots"`
	}{
		PowercycledBots: bots,
	}
	if err := json.NewEncoder(&body).Encode(toReport); err != nil {
		return skerr.Wrapf(err, "encoding JSON")
	}
	_, err := c.Post(daemonURL, "application/json", &body)
	return skerr.Wrapf(err, "posting to %s", daemonURL)
}
