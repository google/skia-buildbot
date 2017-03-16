package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	configFile  = flag.String("conf", "./powercycle.toml", "TOML file with device configuration.")
	powerCycle  = flag.Bool("power_cycle", true, "Powercycle the given dives.")
	listDev     = flag.Bool("list_devices", false, "List the available devices.")
	powerOutput = flag.String("power_output", "", "Filename where to write power stats.")
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

	PowerUsage() (*GroupPowerUsage, error)
}

type GroupPowerUsage struct {
	TS    time.Time
	Stats map[string]*PowerStat
}

type PowerStat struct {
	Ampere float32
	Volt   float32
	Watt   float32
}

func main() {
	common.Init()
	devGroup, err := DeviceGroupFromTomlFile(*configFile)
	if err != nil {
		sklog.Fatalf("Unable to parse config file.  Got error: %s", err)
	}

	if *listDev {
		printHelp(os.Args[0], devGroup)
	} else if *powerOutput != "" {
		tailPower(devGroup, *powerOutput)
	}

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

func tailPower(devGroup DeviceGroup, outputPath string) {
	f, err := os.Create(outputPath)
	if err != nil {
		sklog.Fatalf("Unable to open file '%s': Go error: %s", outputPath, err)
	}
	writer := csv.NewWriter(f)

	// Catch Ctrl-C to flush the file.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		sklog.Infof("Closing cvs file.")
		sklog.Flush()
		writer.Flush()
		util.LogErr(f.Close())
		os.Exit(0)
	}()

	var ids []string = nil
	for range time.Tick(time.Second * 2) {
		// get power stats
		powerStats, err := devGroup.PowerUsage()
		if err != nil {
			sklog.Errorf("Error getting power stats: %s", err)
			continue
		}

		if ids == nil {
			ids = make([]string, 0, len(powerStats.Stats))
			for id := range powerStats.Stats {
				ids = append(ids, id)
			}
			sort.Strings(ids)

			recs := make([]string, 0, len(ids)*3+1)
			recs = append(recs, "time")
			for _, id := range ids {
				recs = append(recs, id+"-A")
				recs = append(recs, id+"-V")
				recs = append(recs, id+"-W")
			}
			writer.Write(recs)
		}

		recs := make([]string, 0, len(ids)*3+1)
		recs = append(recs, powerStats.TS.String())
		var stats *PowerStat
		var ok bool
		for _, id := range ids {
			stats, ok = powerStats.Stats[id]
			if !ok {
				sklog.Errorf("Unable to find expected id: %s", id)
				break
			}
			recs = append(recs, fmt.Sprintf("%5.3f", stats.Ampere))
			recs = append(recs, fmt.Sprintf("%5.3f", stats.Volt))
			recs = append(recs, fmt.Sprintf("%5.3f", stats.Watt))
		}
		if ok {
			writer.Write(recs)
		}
	}
}
