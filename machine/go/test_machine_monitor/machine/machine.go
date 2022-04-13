// Package machine is for interacting with the machine state server. See //machine.
package machine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine"
	changeSource "go.skia.org/infra/machine/go/machine/change/source"
	eventSink "go.skia.org/infra/machine/go/machine/event/sink"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/machineserver/rpc"
	"go.skia.org/infra/machine/go/test_machine_monitor/adb"
	"go.skia.org/infra/machine/go/test_machine_monitor/ios"
	"go.skia.org/infra/machine/go/test_machine_monitor/ssh"
	"go.skia.org/infra/machine/go/test_machine_monitor/swarming"
	"golang.org/x/oauth2/google"
)

const (
	interrogateDuration = 30 * time.Second

	// Recipes require some way to know what the user and ip address are of the device they are
	// talking to. The existing (and easiest) way is to write a file that they know to read.
	// That file is /tmp/ssh_machine.json. The file must be valid JSON and have a key called
	// user_ip that is a string (see //infra/bots/recipe_modules/flavor/ssh.py in the skia repo)
	defaultSSHMachineFileLocation = "/tmp/ssh_machine.json"

	// How often we should poll machines.skia.org for an updated Description.
	descriptionPollDuration = time.Minute
)

var (
	// urlExpansionRegex is used to replace gorilla mux URL variables with
	// values.
	urlExpansionRegex = regexp.MustCompile("{.*}")
)

// Machine is the interface to the machine state server. See //machine.
type Machine struct {
	// An authenticated http client that can talk to the machines.skia.org frontend.
	client *http.Client

	// An absolute URL used to retrieve this machines Description.
	machineDescriptionURL string

	// eventSink is how we send machine.Events to the machine state server.
	eventSink eventSink.Sink

	// changeSource emits events when the machine Description has changed on the
	// server.
	changeSource changeSource.Source

	// adb makes calls to the adb server.
	adb adb.Adb

	// ios is an interface through which we talk to iOS devices.
	ios ios.IOS

	// ssh is an abstraction around an ssh executor
	ssh ssh.SSH

	// MachineID is the swarming id of the machine.
	MachineID string

	// Version of test_machine_monitor being run.
	Version string

	// startTime is the time when this machine started running.
	startTime time.Time

	// Metrics
	interrogateTimer               metrics2.Float64SummaryMetric
	interrogateAndSendFailures     metrics2.Counter
	descriptionWatchArrivalCounter metrics2.Counter

	// startSwarming is true if test_machine_monitor was used to launch Swarming.
	startSwarming bool

	// runningTask is true if the machine is currently running a swarming task.
	runningTask bool

	// mutex protects the description due to the fact it will be updated asynchronously via
	// the firestore snapshot query.
	mutex sync.Mutex

	// description is provided by the machine state server. This tells us what
	// to tell swarming, what our current mode is, etc.
	description rpc.FrontendDescription

	// sshMachineLocation is the name and path of the file to write the JSON data that specifies
	// to recipes how to communicate with the device under test.
	sshMachineLocation string
}

// New return an instance of *Machine.
func New(ctx context.Context, local bool, instanceConfig config.InstanceConfig, version string, startSwarming bool, machineServerHost string) (*Machine, error) {

	sink, err := eventSink.New(ctx, local, instanceConfig)
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

	// Construct the URL need to retrieve this machines Description.
	u, err := url.Parse(machineServerHost)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse machineserver flag: %s", machineServerHost)
	}
	u.Path = urlExpansionRegex.ReplaceAllLiteralString(rpc.MachineDescriptionURL, machineID)

	ts, err := google.DefaultTokenSource(ctx, "email")
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create tokensource.")
	}

	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().WithoutRetries().Client()

	changeSource, err := changeSource.New(ctx, local, instanceConfig.DescriptionChangeSource, machineID)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create changeSource.")
	}

	return &Machine{
		client:                         httpClient,
		machineDescriptionURL:          u.String(),
		eventSink:                      sink,
		changeSource:                   changeSource,
		adb:                            adb.New(),
		ios:                            ios.New(),
		ssh:                            ssh.ExeImpl{},
		sshMachineLocation:             defaultSSHMachineFileLocation,
		MachineID:                      machineID,
		Version:                        version,
		startTime:                      now.Now(ctx),
		startSwarming:                  startSwarming,
		interrogateTimer:               metrics2.GetFloat64SummaryMetric("test_machine_monitor_interrogate_timer", map[string]string{"machine": machineID}),
		interrogateAndSendFailures:     metrics2.GetCounter("test_machine_monitor_interrogate_and_send_errors", map[string]string{"machine": machineID}),
		descriptionWatchArrivalCounter: metrics2.GetCounter("test_machine_monitor_description_watch_arrival", map[string]string{"machine": machineID}),
	}, nil
}

// interrogate the machine we are running on for state-related information. Compile that into a
// machine.Event and return it.
func (m *Machine) interrogate(ctx context.Context) (machine.Event, error) {
	defer timer.NewWithSummary("interrogate", m.interrogateTimer).Stop()

	ret := machine.NewEvent()
	ret.Host.Name = m.MachineID
	ret.Host.Version = m.Version
	ret.Host.StartTime = m.startTime
	ret.RunningSwarmingTask = m.runningTask
	ret.LaunchedSwarming = m.startSwarming
	var err error = nil

	switch m.description.AttachedDevice {
	case machine.AttachedDeviceSSH:
		var ce machine.ChromeOS
		if ce, err = m.tryInterrogatingChromeOSDevice(ctx); err == nil {
			sklog.Infof("Successful communication with ChromeOS device: %#v", ce)
			ret.ChromeOS = ce
		}
	case machine.AttachedDeviceAdb:
		var ae machine.Android
		if ae, err = m.tryInterrogatingAndroidDevice(ctx); err == nil {
			sklog.Infof("Successful communication with abd device: %#v", ae)
			ret.Android = ae
		}
	case machine.AttachedDeviceiOS:
		var ie machine.IOS
		if ie, err = m.tryInterrogatingIOSDevice(ctx); err == nil {
			sklog.Infof("Successful communication with iOS device: %#v", ie)
			ret.IOS = ie
		}
	case machine.AttachedDeviceNone:
		sklog.Infof("No attached device expected.")
	default:
		sklog.Error("Unhandled type of machine.AttachedDevice.")

	}

	return ret, skerr.Wrap(err)
}

// interrogateAndSend gathers the state for this machine and sends it to the
// sink. Of note, this does not directly determine what dimensions this machine
// should have. The machine server that listens to the events will determine the
// dimensions based on the reported state and any information it has from other
// sources (e.g. human-supplied details, previously attached devices)
func (m *Machine) interrogateAndSend(ctx context.Context) error {
	event, err := m.interrogate(ctx)
	if err != nil {
		// Don't return an error here, otherwise Start() will always return err,
		// for example, if an Android device is missing, and that's a fatal
		// error.
		sklog.Errorf("Failed to interrogate: %s", err)
	}
	if err := m.eventSink.Send(ctx, event); err != nil {
		return skerr.Wrapf(err, "Failed to send interrogation step.")
	}
	return nil
}

// Start the background processes that send events to the sink and watch for
// changes to the Description.
func (m *Machine) Start(ctx context.Context) error {

	// First do a single steps of interrogating to make sure that sending the
	// event works. We don't do the same for the Description since this may be a
	// new machine and retrieveDescription could fail.
	if err := m.interrogateAndSend(ctx); err != nil {
		return skerr.Wrap(err)
	}

	go m.startInterrogateLoop(ctx)

	go m.startDescriptionWatch(ctx)

	return nil
}

// Start a loop that scans for local devices and sends pubsub events with all
// the data every 30s.
func (m *Machine) startInterrogateLoop(ctx context.Context) {
	util.RepeatCtx(ctx, interrogateDuration, func(ctx context.Context) {
		if err := m.interrogateAndSend(ctx); err != nil {
			m.interrogateAndSendFailures.Inc(1)
			sklog.Errorf("interrogateAndSend failed: %s", err)
		}
	})
}

// retrieveDescription stores and updates the machine Description in m.description.
func (m *Machine) retrieveDescription(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", m.machineDescriptionURL, nil)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create HTTP request")
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return skerr.Wrapf(err, "Failed to retrieve description from %q", m.machineDescriptionURL)
	}
	var desc rpc.FrontendDescription
	if err := json.NewDecoder(resp.Body).Decode(&desc); err != nil {
		return skerr.Wrapf(err, "Failed to decode description from %q", m.machineDescriptionURL)
	}
	m.UpdateDescription(desc)
	m.descriptionWatchArrivalCounter.Inc(1)
	return nil
}

// startDescriptionWatch starts a loop that continually looks for updates to the
// machine Description. This function does not return unless the context is
// cancelled.
func (m *Machine) startDescriptionWatch(ctx context.Context) {
	changeCh := m.changeSource.Start(ctx)
	tickCh := time.NewTicker(descriptionPollDuration).C
	for {
		select {
		case <-changeCh:
			if err := m.retrieveDescription(ctx); err != nil {
				sklog.Errorf("Event driven retrieveDescription failed: %s", err)
			}
		case <-tickCh:
			if err := m.retrieveDescription(ctx); err != nil {
				sklog.Errorf("Timer driven retrieveDescription failed: %s", err)
			}
		case <-ctx.Done():
			sklog.Errorf("context cancelled")
			return
		}
	}
}

// UpdateDescription applies any change in behavior based on the new given description. This
// impacts what we tell Swarming, what mode we are in, if we should communicate with a device
// via SSH, etc.
func (m *Machine) UpdateDescription(desc rpc.FrontendDescription) {
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
// The message to display for Maintenance Mode is also returned.
func (m *Machine) GetMaintenanceMode() (bool, string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	ret := "Maintenance Mode"
	if m.description.Note.Message != "" {
		ret = m.description.Note.Message
	}
	return m.description.Mode == machine.ModeRecovery || m.description.Mode == machine.ModeMaintenance, ret
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
	shouldRebootAndroid := len(m.description.Dimensions[machine.DimAndroidDevices]) > 0
	shouldRebootIOS := util.In("iOS", m.description.Dimensions[machine.DimOS])
	sshUserIP := m.description.SSHUserIP
	m.mutex.Unlock()

	if shouldRebootAndroid {
		return m.adb.Reboot(ctx)
	} else if shouldRebootIOS {
		return m.ios.Reboot(ctx)
	} else if sshUserIP != "" {
		return m.rebootChromeOS(ctx, sshUserIP)
	}
	sklog.Info("No attached device to reboot.")
	return nil
}

// tryInterrogatingAndroidDevice attempts to communicate with an Android device using the
// adb interface. If there is one attached, this function returns true and the information gathered
// (which can be partially filled out). If there is not a device attached, false is returned.
func (m *Machine) tryInterrogatingAndroidDevice(ctx context.Context) (machine.Android, error) {
	metrics2.GetCounter("test_machine_monitor_interrogate_device_type", map[string]string{
		"machine": m.MachineID,
		"type":    "android",
	}).Inc(1)
	sklog.Info("tryInterrogatingAndroidDevice")
	var ret machine.Android

	if err := m.adb.EnsureOnline(ctx); err != nil {
		sklog.Warningf("No Android device is available: %s", err)
		return ret, skerr.Wrapf(err, "No Android device is available")
	}
	if uptime, err := m.adb.Uptime(ctx); err != nil {
		return ret, skerr.Wrapf(err, "Failed to read uptime - assuming there is no Android device attached.")
	} else {
		ret.Uptime = uptime
	}

	props, err := m.adb.RawProperties(ctx)
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to read android properties.")
	}
	ret.GetProp = props

	if battery, err := m.adb.RawDumpSys(ctx, "battery"); err == nil {
		ret.DumpsysBattery = battery
	} else {
		sklog.Warningf("Failed to read android battery status: %s", err)
	}

	if thermal, err := m.adb.RawDumpSys(ctx, "thermalservice"); err == nil {
		ret.DumpsysThermalService = thermal
	} else {
		sklog.Warningf("Failed to read android thermal status.", err)
	}
	return ret, nil
}

// tryInterrogatingIOSDevice attempts to communicate with an attached iOS device. If there is one,
// this function returns true and the information gathered (which can be partially filled out). If
// there is not a device attached, it returns false, and the other return value is undefined. If
// multiple devices are attached, an arbitrary one is chosen.
func (m *Machine) tryInterrogatingIOSDevice(ctx context.Context) (machine.IOS, error) {
	metrics2.GetCounter("test_machine_monitor_interrogate_device_type", map[string]string{
		"machine": m.MachineID,
		"type":    "ios",
	}).Inc(1)
	sklog.Info("tryInterrogatingIOSDevice")

	var ret machine.IOS
	var err error

	deviceType, err := m.ios.DeviceType(ctx)
	if err != nil {
		return ret, skerr.Wrap(err)
	}
	ret.DeviceType = deviceType

	if osVersion, err := m.ios.OSVersion(ctx); err != nil {
		sklog.Warningf("Failed to read iOS version, though we managed to read the device type: %s", err)
	} else {
		ret.OSVersion = osVersion
	}

	battery, err := m.ios.BatteryLevel(ctx)
	if err != nil {
		sklog.Warningf("Failed to read iOS device battery level, though we managed to read its device type: %s", err)
	}
	ret.Battery = battery

	return ret, nil
}

var (
	chromeOSReleaseRegex   = regexp.MustCompile(`CHROMEOS_RELEASE_VERSION=(\S+)`)
	chromeOSMilestoneRegex = regexp.MustCompile(`CHROMEOS_RELEASE_CHROME_MILESTONE=(\S+)`)
	chromeOSTrackRegex     = regexp.MustCompile(`CHROMEOS_RELEASE_TRACK=(\S+)`)
)

func (m *Machine) tryInterrogatingChromeOSDevice(ctx context.Context) (machine.ChromeOS, error) {
	metrics2.GetCounter("test_machine_monitor_interrogate_device_type", map[string]string{
		"machine": m.MachineID,
		"type":    "chromeos",
	}).Inc(1)
	sklog.Info("tryInterrogatingChromeOSDevice")

	var ret machine.ChromeOS
	if m.description.SSHUserIP == "" {
		return ret, skerr.Fmt("no machine.SSHUserIP supplied")
	}
	uptime, err := m.ssh.Run(ctx, m.description.SSHUserIP, "cat", "/proc/uptime")
	if err != nil {
		return ret, skerr.Wrapf(err, "Could not read ChromeOS uptime - assuming there is no ChromeOS device attached")
	}
	u := strings.Split(uptime, " ")[0]
	if f, err := strconv.ParseFloat(u, 64); err != nil {
		return ret, skerr.Wrapf(err, "Invalid /proc/uptime format: %q", uptime)
	} else {
		ret.Uptime = time.Duration(f * float64(time.Second))
	}

	lsbReleaseContents, err := m.ssh.Run(ctx, m.description.SSHUserIP, "cat", "/etc/lsb-release")
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to read lsb-release - assuming there is no ChromeOS device attached")
	}
	if match := chromeOSReleaseRegex.FindStringSubmatch(lsbReleaseContents); match != nil {
		ret.ReleaseVersion = match[1]
	}
	if match := chromeOSMilestoneRegex.FindStringSubmatch(lsbReleaseContents); match != nil {
		ret.Milestone = match[1]
	}
	if match := chromeOSTrackRegex.FindStringSubmatch(lsbReleaseContents); match != nil {
		ret.Channel = match[1]
	}
	if ret.ReleaseVersion == "" && ret.Milestone == "" && ret.Channel == "" {
		return ret, skerr.Wrapf(err, "Could not find ChromeOS data in /etc/lsb-release. Are we sure this is the right IP?\n%s", lsbReleaseContents)
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
		return ret, skerr.Wrapf(err, "Could not write SSH info to %s", m.sshMachineLocation)
	}
	return ret, nil
}

// rebootChromeOS reboots the ChromeOS device attached via SSH.
func (m *Machine) rebootChromeOS(ctx context.Context, userIP string) error {
	out, err := m.ssh.Run(ctx, userIP, "reboot")
	if err != nil {
		sklog.Warningf("Could not reboot ChromeOS device %s: %s", m.description.SSHUserIP, out)
		return skerr.Wrap(err)
	}
	return nil
}
