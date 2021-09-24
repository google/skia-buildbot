// Package machine is for interacting with the machine state server. See //machine.
package machine

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/sink"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/test_machine_monitor/adb"
	"go.skia.org/infra/machine/go/test_machine_monitor/ssh"
	"go.skia.org/infra/machine/go/test_machine_monitor/swarming"
)

const (
	interrogateDuration = 30 * time.Second

	// Recipes require some way to know what the user and ip address are of the device they are
	// talking to. The existing (and easiest) way is to write a file that they know to read.
	// That file is /tmp/ssh_machine.json. The file must be valid JSON and have a key called
	// user_ip that is a string (see //infra/bots/recipe_modules/flavor/ssh.py in the skia repo)
	defaultSSHMachineFileLocation = "/tmp/ssh_machine.json"
)

// Machine is the interface to the machine state server. See //machine.
type Machine struct {
	// store is how we get our dimensions and status updates from the machine state server.
	store store.Store

	// sink is how we send machine.Events to the the machine state server.
	sink sink.Sink

	// adb makes calls to the adb server.
	adb adb.Adb

	// ssh is an abstraction around an ssh executor
	ssh ssh.SSH

	// MachineID is the swarming id of the machine.
	MachineID string

	// Version of test_machine_monitor being run.
	Version string

	// startTime is the time when this machine started running.
	startTime time.Time

	// Metrics
	interrogateTimer           metrics2.Float64SummaryMetric
	interrogateAndSendFailures metrics2.Counter
	storeWatchArrivalCounter   metrics2.Counter

	// startSwarming is true if test_machine_monitor was used to launch Swarming.
	startSwarming bool

	// runningTask is true if the machine is currently running a swarming task.
	runningTask bool

	// mutex protects the description due to the fact it will be updated asynchronously via
	// the firestore snapshot query.
	mutex sync.Mutex

	// description is provided by the machine state server. This tells us what
	// to tell swarming, what our current mode is, etc.
	description machine.Description

	// sshMachineLocation is the name and path of the file to write the JSON data that specifies
	// to recipes how to communicate with the device under test.
	sshMachineLocation string
}

// New return an instance of *Machine.
func New(ctx context.Context, local bool, instanceConfig config.InstanceConfig, startTime time.Time, version string, startSwarming bool) (*Machine, error) {
	store, err := store.NewFirestoreImpl(ctx, false, instanceConfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to build store instance.")
	}
	sink, err := sink.New(ctx, local, instanceConfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to build sink instance.")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not determine hostname.")
	}
	machineID := os.Getenv(swarming.SwarmingBotIDEnvVar)
	if machineID == "" {
		// Fall back to hostname so we can track machines that
		// test_machine_monitor is running on that don't also run Swarming.
		machineID = hostname
	}

	return &Machine{
		store:                      store,
		sink:                       sink,
		adb:                        adb.New(),
		ssh:                        ssh.ExeImpl{},
		sshMachineLocation:         defaultSSHMachineFileLocation,
		MachineID:                  machineID,
		Version:                    version,
		startTime:                  startTime,
		startSwarming:              startSwarming,
		interrogateTimer:           metrics2.GetFloat64SummaryMetric("bot_config_machine_interrogate_timer", map[string]string{"machine": machineID}),
		interrogateAndSendFailures: metrics2.GetCounter("bot_config_machine_interrogate_and_send_errors", map[string]string{"machine": machineID}),
		storeWatchArrivalCounter:   metrics2.GetCounter("bot_config_machine_store_watch_arrival", map[string]string{"machine": machineID}),
	}, nil
}

// interrogate the machine we are running on for state-related information. It compiles that into
// a machine.Event and returns it.
func (m *Machine) interrogate(ctx context.Context) machine.Event {
	defer timer.NewWithSummary("interrogate", m.interrogateTimer).Stop()

	ret := machine.NewEvent()
	ret.Host.Name = m.MachineID
	ret.Host.Version = m.Version
	ret.Host.StartTime = m.startTime
	ret.RunningSwarmingTask = m.runningTask
	ret.LaunchedSwarming = m.startSwarming

	if ce, ok := m.tryInterrogatingChromeOSDevice(ctx); ok {
		sklog.Infof("Successful communication with ChromeOS device: %#v", ce)
		ret.ChromeOS = ce
	} else if ae, ok := m.tryInterrogatingAndroidDevice(ctx); ok {
		sklog.Infof("Successful communication with Android device: %#v", ae)
		ret.Android = ae
	} else if ie, ok := m.tryInterrogatingIOSDevice(ctx); ok {
		sklog.Infof("Successful communication with iOS device: %#v", ie)
		ret.IOS = ie
	} else {
		sklog.Infof("No attached device found")
	}

	return ret
}

// interrogateAndSend gathers the state for this machine and sends it to the sink. Of note, this
// does not directly determine what dimensions this machine should have. The machine server that
// listens to the events will determine the dimensions based on the reported state and any
// information it has from other sources (e.g. human-supplied details, previously attached devices)
func (m *Machine) interrogateAndSend(ctx context.Context) error {
	event := m.interrogate(ctx)
	if err := m.sink.Send(ctx, event); err != nil {
		return skerr.Wrapf(err, "Failed to send interrogation step.")
	}
	return nil
}

// Start the background processes that send events to the sink and watch for
// firestore changes.
func (m *Machine) Start(ctx context.Context) error {
	m.startStoreWatch(ctx)

	if err := m.interrogateAndSend(ctx); err != nil {
		return skerr.Wrap(err)
	}

	// Start a loop that scans for local devices and sends pubsub events with all the
	// data every 30s.
	go util.RepeatCtx(ctx, interrogateDuration, func(ctx context.Context) {
		if err := m.interrogateAndSend(ctx); err != nil {
			m.interrogateAndSendFailures.Inc(1)
			sklog.Errorf("interrogateAndSend failed: %s", err)
		}
	})

	return nil
}

// startStoreWatch starts a loop that does a firestore onsnapshot watcher that gets the dims
// and state we should be reporting to swarming. It blocks until the first description is loaded.
func (m *Machine) startStoreWatch(ctx context.Context) {
	c := m.store.Watch(ctx, m.MachineID)
	desc := <-c
	m.storeWatchArrivalCounter.Inc(1)
	sklog.Infof("Loaded existing description from FS: %#v", desc)
	m.UpdateDescription(desc)
	go func() {
		for desc := range c {
			m.storeWatchArrivalCounter.Inc(1)
			m.UpdateDescription(desc)
		}
	}()
}

// UpdateDescription applies any change in behavior based on the new given description. This
// impacts what we tell Swarming, what mode we are in, if we should communicate with a device
// via SSH, etc.
func (m *Machine) UpdateDescription(desc machine.Description) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.description = desc
}

// DimensionsForSwarming returns the dimensions that should be reported to swarming.
func (m *Machine) DimensionsForSwarming() machine.SwarmingDimensions {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.description.Dimensions
}

// GetMaintenanceMode returns true if the machine should report to Swarming that it is
// in maintenance mode. Swarming does not have a "recovery" mode, so we group that in.
func (m *Machine) GetMaintenanceMode() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.description.Mode == machine.ModeRecovery || m.description.Mode == machine.ModeMaintenance
}

// SetIsRunningSwarmingTask records if a swarming task is being run.
func (m *Machine) SetIsRunningSwarmingTask(isRunning bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.runningTask = isRunning
}

// IsRunningSwarmingTask returns true is a swarming task is currently running.
func (m *Machine) IsRunningSwarmingTask() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.runningTask
}

// RebootDevice reboots the attached device.
func (m *Machine) RebootDevice(ctx context.Context) error {
	m.mutex.Lock()
	shouldReboot := len(m.description.Dimensions[machine.DimAndroidDevices]) > 0
	m.mutex.Unlock()

	if shouldReboot {
		return m.adb.Reboot(ctx)
	}
	sklog.Info("No attached device to reboot.")
	return nil
}

// tryInterrogatingAndroidDevice attempts to communicate with an Android device using the
// adb interface. If there is one attached, this function returns true and the information gathered
// (which can be partially filled out). If there is not a device attached, false is returned.
func (m *Machine) tryInterrogatingAndroidDevice(ctx context.Context) (machine.Android, bool) {
	var ret machine.Android

	if err := m.adb.EnsureOnline(ctx); err != nil {
		sklog.Warningf("No Android device is available: %s", err)
		return ret, false
	}
	if uptime, err := m.adb.Uptime(ctx); err != nil {
		sklog.Warningf("Failed to read uptime - assuming there is no Android device attached: %s", err)
		return ret, false // Assume there is no Android device attached.
	} else {
		ret.Uptime = uptime
	}

	if props, err := m.adb.RawProperties(ctx); err != nil {
		sklog.Warningf("Failed to read android properties: %s", err)
	} else {
		ret.GetProp = props
	}

	if battery, err := m.adb.RawDumpSys(ctx, "battery"); err != nil {
		sklog.Warningf("Failed to read android battery status: %s", err)
	} else {
		ret.DumpsysBattery = battery
	}

	if thermal, err := m.adb.RawDumpSys(ctx, "thermalservice"); err != nil {
		sklog.Warningf("Failed to read android thermal status: %s", err)
	} else {
		ret.DumpsysThermalService = thermal
	}
	return ret, true
}

func (m *Machine) tryInterrogatingIOSDevice(_ context.Context) (machine.IOS, bool) {
	// TODO(erikrose)
	return machine.IOS{}, false
}

var (
	chromeOSReleaseRegex   = regexp.MustCompile(`CHROMEOS_RELEASE_VERSION=(\S+)`)
	chromeOSMilestoneRegex = regexp.MustCompile(`CHROMEOS_RELEASE_CHROME_MILESTONE=(\S+)`)
	chromeOSTrackRegex     = regexp.MustCompile(`CHROMEOS_RELEASE_TRACK=(\S+)`)
)

func (m *Machine) tryInterrogatingChromeOSDevice(ctx context.Context) (machine.ChromeOS, bool) {
	if m.description.SSHUserIP == "" {
		return machine.ChromeOS{}, false
	}
	rv := machine.ChromeOS{}
	uptime, err := m.ssh.Run(ctx, m.description.SSHUserIP, "cat", "/proc/uptime")
	if err != nil {
		sklog.Warningf("Could not read ChromeOS uptime %s - assuming there is no ChromeOS device attached", err)
		return machine.ChromeOS{}, false
	} else {
		u := strings.Split(uptime, " ")[0]
		if f, err := strconv.ParseFloat(u, 64); err != nil {
			sklog.Warningf("Invalid /proc/uptime format: %q", uptime)
		} else {
			rv.Uptime = time.Duration(f * float64(time.Second))
		}
	}

	lsbReleaseContents, err := m.ssh.Run(ctx, m.description.SSHUserIP, "cat", "/etc/lsb-release")
	if err != nil {
		sklog.Warningf("Failed to read lsb-release - assuming there is no ChromeOS device attached: %s", err)
		return machine.ChromeOS{}, false
	}
	if match := chromeOSReleaseRegex.FindStringSubmatch(lsbReleaseContents); match != nil {
		rv.ReleaseVersion = match[1]
	}
	if match := chromeOSMilestoneRegex.FindStringSubmatch(lsbReleaseContents); match != nil {
		rv.Milestone = match[1]
	}
	if match := chromeOSTrackRegex.FindStringSubmatch(lsbReleaseContents); match != nil {
		rv.Channel = match[1]
	}
	if rv.ReleaseVersion == "" && rv.Milestone == "" && rv.Channel == "" {
		sklog.Errorf("Could not find ChromeOS data in /etc/lsb-release. Are we sure this is the right IP?\n%s", lsbReleaseContents)
		return machine.ChromeOS{}, false
	}
	// Now that we know we can connect to the SSH machine, make sure recipes can as well.
	err = util.WithWriteFile(m.sshMachineLocation, func(w io.Writer) error {
		type sshMachineInfo struct {
			Comment string
			UserIP  string `json:"user_ip"`
		}
		toWrite := sshMachineInfo{
			Comment: "This file is written to by test_machine_monitor. Do not edit by hand.",
			UserIP:  m.description.SSHUserIP,
		}
		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		return e.Encode(toWrite)
	})
	if err != nil {
		sklog.Errorf("Could not write SSH info to %s: %s", m.sshMachineLocation, err)
		return machine.ChromeOS{}, false
	}
	return rv, true
}
