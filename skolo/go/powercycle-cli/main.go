package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/skolo/go/powercycle"
)

var (
	configFile  = flag.String("conf", "/etc/powercycle.json5", "JSON5 file with device configuration.")
	delay       = flag.Int("delay", 0, "Any value > 0 overrides the default duration (in sec) between turning the port off and on.")
	connect     = flag.Bool("connect", false, "Connect to all the powercycle hosts and verify they are attached computing attached devices.")
	list        = flag.Bool("l", false, "List the available devices and exit.")
	powerOutput = flag.String("power_output", "", "Continously poll power usage and write it to the given file. Press ^C to exit.")
	sampleRate  = flag.Duration("power_sample_rate", 2*time.Second, "Time delay between capturing power usage.")

	autoFix    = flag.Bool("auto_fix", false, "Fetch the list of down bots/devices from power.skia.org and reboot those.")
	autoFixURL = flag.String("auto_fix_url", "https://power.skia.org/down_bots", "Url at which to grab the list of down bots/devices.")

	daemonURL = flag.String("daemon_url", "http://localhost:9210/powercycled_bots", "The (probably localhost) URL at which the powercycle daemon is located.")
)

func main() {
	common.Init()
	args := flag.Args()
	if !*connect && !*autoFix && len(args) == 0 {
		sklog.Info("Skipping connection test. Use --connect flag to force connection testing.")
	} else {
		// Force connect to be on because we will be powercycling devices. If we try to
		// powercycle without connecting first, the DeviceGroups won't be properly initialized.
		*connect = true
	}
	devGroup, err := powercycle.DeviceGroupFromJson5File(*configFile, *connect)
	if err != nil {
		sklog.Fatalf("Unable to parse config file.  Got error: %s", err)
	}

	if *list {
		listDevices(devGroup, 0)
	} else if *powerOutput != "" {
		if *sampleRate <= 0 {
			sklog.Fatal("Non-positive sample rate provided.")
		}
	}

	if *autoFix {
		args, err = GetAutoFixCandidates(*autoFixURL)
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

	// Check if the device ids are valid.
	validDeviceIds := devGroup.DeviceIDs()
	for _, arg := range args {
		if !util.In(arg, validDeviceIds) {
			sklog.Errorf("Invalid device ID.")
			listDevices(devGroup, 1)
		}
	}

	for _, deviceID := range args {
		if err := devGroup.PowerCycle(deviceID, time.Duration(*delay)*time.Second); err != nil {
			sklog.Fatalf("Unable to power cycle device %s. Got error: %s", deviceID, err)
		}
	}
	if err := reportToDaemon(args); err != nil {
		sklog.Fatalf("Could not report powercyling through daemon: %s", err)
	}
	sklog.Infof("Power cycle successful. All done.")
	sklog.Flush()
}

// listDevices prints out the devices it know about. This implies that
// the devices have been contacted and passed a ping test.
func listDevices(devGroup powercycle.DeviceGroup, exitCode int) {
	sklog.Errorf("Valid device IDs are:\n\n")
	for _, id := range devGroup.DeviceIDs() {
		sklog.Errorf("    %s\n", id)
	}
	os.Exit(exitCode)
}

func reportToDaemon(bots []string) error {
	if *daemonURL == "" {
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
		return fmt.Errorf("Problem encoding json: %s", err)
	}
	_, err := c.Post(*daemonURL, "application/json", &body)
	return err
}
