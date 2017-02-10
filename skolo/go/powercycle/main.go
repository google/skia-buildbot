package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// DeviceGroup describes a set of devices that can all be
// controlled together. Any switch or power strip needs to
// implement this interface.
type DeviceGroup interface {
	// DeviceIDs returns a list of strings that uniquely identify
	// the devices that can be controlled through this group.
	DeviceIDs() []string

	// PowerCycle turns the device off for a reasonable amount of time
	// (i.e. 10 seconds) and then turns it back on. The duration should
	// be chosen by the implemenation to ensure that all residual charges
	// leave the device.
	PowerCycle(devID string) error
}

func main() {
	devGroup, err := NewMPowerClient()
	if err != nil {
		log.Fatalf("Unable to create mPower client. Got error: %s", err)
	}

	// Set up an extended usage function.
	flag.Usage = func() {
		printHelp(os.Args[0], devGroup)
	}

	common.Init()

	// No device id given.
	args := flag.Args()
	if len(args) == 0 {
		printHelp(os.Args[0], devGroup)
	}

	// Check if the device ids are valid.
	validDeviceIds := devGroup.DeviceIDs()
	for _, arg := range args {
		if !util.In(arg, validDeviceIds) {
			printHelp(os.Args[0], devGroup)
		}
	}

	for _, deviceID := range args {
		if err := devGroup.PowerCycle(deviceID); err != nil {
			sklog.Fatalf("Unable to power cycle device %s. Got error: %s", deviceID, err)
		}

		sklog.Infof("Power cycle successful. All done.")
		sklog.Flush()
	}
}

func printHelp(appName string, devGroup DeviceGroup) {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] device_id[, device_id, ...]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Valid device IDs are:\n\n")
	for _, id := range devGroup.DeviceIDs() {
		fmt.Fprintf(os.Stderr, "    %s\n", id)
	}

	fmt.Fprintf(os.Stderr, "\n\nOptions:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\n\n")
	os.Exit(1)
}
