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
	"go.skia.org/infra/skolo/go/powercycle"
)

const (
	// Duration after which to flush powerusage stats to disk.
	FLUSH_POWER_USAGE = time.Minute
)

var (
	configFile  = flag.String("conf", "/etc/powercycle.json5", "JSON5 file with device configuration.")
	delay       = flag.Int("delay", 0, "Any value > 0 overrides the default duration (in sec) between turning the port off and on.")
	connect     = flag.Bool("connect", false, "Connect to all the powercycle hosts and verify they are attached computing attached devices.")
	list        = flag.Bool("l", false, "List the available devices and exit.")
	powerCycle  = flag.Bool("power_cycle", true, "Powercycle the given devices.")
	powerOutput = flag.String("power_output", "", "Continously poll power usage and write it to the given file. Press ^C to exit.")
	sampleRate  = flag.Duration("power_sample_rate", 2*time.Second, "Time delay between capturing power usage.")

	autoFix    = flag.Bool("auto_fix", false, "Fetch the list of down bots/devices from power.skia.org and reboot those.")
	autoFixURL = flag.String("auto_fix_url", "https://power.skia.org/down_bots", "Url at which to grab the list of down bots/devices.")
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
		tailPower(devGroup, *powerOutput, *sampleRate)
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
		sklog.Info("Use Ctrl+C to cancel if this is unwanted/wroasdfsdfsdng.")
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

		sklog.Infof("Power cycle successful. All done.")
		sklog.Flush()
	}
}

// listDevices prints out the devices it know about. This implies that
// the devices have been contacted and passed a ping test.
func listDevices(devGroup powercycle.DeviceGroup, exitCode int) {
	fmt.Fprintf(os.Stderr, "Valid device IDs are:\n\n")
	for _, id := range devGroup.DeviceIDs() {
		fmt.Fprintf(os.Stderr, "    %s\n", id)
	}
	os.Exit(exitCode)
}

// tailPower continually polls the power usage and writes the values in
// a CSV file.
func tailPower(devGroup powercycle.DeviceGroup, outputPath string, sampleRate time.Duration) {
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
	lastFlush := time.Now()
	for range time.Tick(sampleRate) {
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
			if err := writer.Write(recs); err != nil {
				sklog.Errorf("Error writing CSV records: %s", err)
			}
		}

		recs := make([]string, 0, len(ids)*3+1)
		recs = append(recs, powerStats.TS.String())
		var stats *powercycle.PowerStat
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
			if err := writer.Write(recs); err != nil {
				sklog.Errorf("Error writing CSV records: %s", err)
			}

			if time.Now().Sub(lastFlush) >= FLUSH_POWER_USAGE {
				lastFlush = time.Now()
				writer.Flush()
			}
		}
	}
}
