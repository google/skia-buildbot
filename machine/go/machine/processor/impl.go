// Package processor does the work of taking incoming events from machines and
// updating the machine state using that information. The goal is to move all
// the logic out of `skia_mobile.py` and into processor.
//
// TODO(jcgregorio) Add support for devices beyond Android.
// TODO(kjlubick,jcgregorio) Use ro.build.fingerprint to catch cases where the
// phone manufacturuers push and update but don't rev the android version.
package processor

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine"
)

const (
	// minBatteryLevel is the minimum percentage that a battery needs to be
	// charged before testing is allowed. Below this the device should be
	// quarantined.
	minBatteryLevel = 30

	badTemperature float64 = -99

	// maxTemperatureC is the highest we want to see a devices temperature. Over
	// this the device should be quarantined.
	maxTemperatureC float64 = 35

	batteryTemperatureKey = "dumpsys_battery"

	// The username for annotations made by the machine server.
	machineUserName = "machines.skia.org"
)

var (
	// proplines is a regex that matches the output of `adb shell getprop`. Which
	// has output that looks like:
	//
	// [ro.product.manufacturer]: [asus]
	// [ro.product.model]: [Nexus 7]
	// [ro.product.name]: [razor]
	proplines = regexp.MustCompile(`(?m)^\[(?P<key>.+)\]:\s*\[(?P<value>.*)\].*$`)

	// batteryLevel is a regex that matches the output of `adb shell dumpsys battery` and looks
	// for the battery level which looks like:
	//
	//    level: 94
	batteryLevel = regexp.MustCompile(`(?m)level:\s+(\d+)\b`)

	// batteryScale is a regex that matches the output of `adb shell dumpsys battery` and looks
	// for the battery scale which looks like:
	//
	//    scale: 100
	batteryScale = regexp.MustCompile(`(?m)scale:\s+(\d+)\b`)

	// batteryTemperature is a regex that matches the output of `adb shell
	// dumpsys battery` and looks for the battery temperature which looks like:
	//
	//      temperature: 280
	batteryTemperature = regexp.MustCompile(`(?m)temperature:\s+(\d+)\b`)

	// thermalServiceTemperature is a regex that matches the temperatures in the
	// output of `adb shell dumpsys thermalservice`, which look like:
	//
	//      Temperature{mValue=28.000002, mType=2, mName=battery, mStatus=0}
	//
	// Note the regex doesn't match negative temperatures so those will be
	// ignored. Some devices set a value of -99.9 for the temp of a device they
	// aren't actually measuring.
	//
	// The groups in the regex are:
	//   1. The temp as a float.
	//   2. The name of the part being measured.
	//   3. The status, which corresponds to different throttling levels.
	//      See https://source.android.com/devices/architecture/hidl/thermal-mitigation#thermal-service
	//
	// The name and status are currently unused.
	thermalServiceTemperature = regexp.MustCompile(`\tTemperature{mValue=([\d\.]+),\s+mType=\d+,\s+mName=([^,]+),\s+mStatus=(\d)`)
)

// ProcessorImpl implements the Processor interface.
type ProcessorImpl struct {
	unknownEventTypeCount metrics2.Counter
	eventsProcessedCount  metrics2.Counter
}

// New returns a new Processor instance.
func New(ctx context.Context) *ProcessorImpl {
	return &ProcessorImpl{
		unknownEventTypeCount: metrics2.GetCounter("machineserver_processor_unknown_event_type"),
		eventsProcessedCount:  metrics2.GetCounter("machineserver_processor_events_processed"),
	}
}

// Process implements the Processor interface.
func (p *ProcessorImpl) Process(ctx context.Context, previous machine.Description, event machine.Event) machine.Description {
	p.eventsProcessedCount.Inc(1)
	if event.EventType != machine.EventTypeRawState {
		p.unknownEventTypeCount.Inc(1)
		sklog.Errorf("Unknown event type: %q", event.EventType)
		return previous
	}
	next := p.processEvent(ctx, previous, event)

	if event.ForcedQuarantine {
		next.IsQuarantined = true
	}

	return next
}

func (p *ProcessorImpl) processEvent(ctx context.Context, previous machine.Description, event machine.Event) machine.Description {
	if event.Android.IsPopulated() {
		return processAndroidEvent(ctx, previous, event)
	} else if event.ChromeOS.IsPopulated() {
		return processChromeOSEvent(ctx, previous, event)
	} else if event.IOS.IsPopulated() {
		return processIOSEvent(ctx, previous, event)
	} else if event.PyOCD.IsPopulated() {
		return processPyOCDEvent(ctx, previous, event)
	} else if event.Standalone.IsPopulated() {
		return processStandaloneEvent(ctx, previous, event)
	}
	return processMissingDeviceEvent(ctx, previous, event)
}

func processAndroidEvent(ctx context.Context, previous machine.Description, event machine.Event) machine.Description {
	machineID := event.Host.Name
	dimensions := dimensionsFromAndroidProperties(parseAndroidProperties(event.Android.GetProp))
	dimensions[machine.DimID] = []string{machineID}
	maintenanceMessages := []string{}

	shouldPowerCycle := false
	if previous.PowerCycle {
		shouldPowerCycle = true
	}

	battery, ok := batteryFromAndroidDumpSys(event.Android.DumpsysBattery)
	if ok {
		if battery < minBatteryLevel {
			maintenanceMessages = append(maintenanceMessages, "Battery low.")
		}
		metrics2.GetInt64Metric("machine_processor_device_battery_level", map[string]string{"machine": machineID}).Update(int64(battery))
	} else {
		sklog.Warningf("Failed to find battery value for %s", machineID)
	}

	temperatures, ok := temperatureFromAndroid(event.Android)
	if ok {
		temperature := findMaxTemperature(temperatures)
		if temperature > maxTemperatureC {
			maintenanceMessages = append(maintenanceMessages, "Too hot.")
		}
		for sensor, temp := range temperatures {
			metrics2.GetFloat64Metric("machine_processor_device_temperature_c", map[string]string{"machine": machineID, "sensor": sensor}).Update(temp)
		}
	}

	ret := previous.Copy()
	ret.Battery = battery
	ret.Temperature = temperatures
	ret.DeviceUptime = int32(event.Android.Uptime.Seconds())
	ret.PowerCycle = shouldPowerCycle
	for k, values := range dimensions {
		ret.Dimensions[k] = values
	}

	ret = handleGeneralFields(ctx, ret, event)
	ret = handleRecoveryMode(ctx, previous, ret, strings.Join(maintenanceMessages, " "))
	return ret
}

// handleGeneralFields extracts general information from the event.
func handleGeneralFields(ctx context.Context, current machine.Description, event machine.Event) machine.Description {
	current.LastUpdated = now.Now(ctx)
	current.RunningSwarmingTask = event.RunningSwarmingTask
	metrics2.GetBoolMetric("machine_processor_running_swarming_task", current.Dimensions.AsMetricsTags()).Update(current.RunningSwarmingTask)
	current.LaunchedSwarming = event.LaunchedSwarming
	current.Version = event.Host.Version
	current.Dimensions[machine.DimTestMachineMonitorVersion] = []string{event.Host.Version}
	return current
}

// handleRecoveryMode records transitions in or out of Recovery mode.
func handleRecoveryMode(ctx context.Context, previous, current machine.Description, recoveryMessage string) machine.Description {
	isRecovering := recoveryMessage != ""

	// If the machine is going into Recovery mode then record the start time.
	// Note that if the machine is currently running a test then the amount of
	// time in recovery will also include some of the test time, but that's the
	// price we pay to avoid a race condition where a test ends and a new test
	// starts before we set Recovery mode.
	if isRecovering && !previous.IsRecovering() {
		current.Recovering = recoveryMessage
		current.RecoveryStart = now.Now(ctx)
		current.Annotation.Timestamp = now.Now(ctx)
		current.Annotation.Message = recoveryMessage
		current.Annotation.User = machineUserName
	}

	// If the machine didn't report an empty battery or other bad condition, move back being
	// available.
	if !isRecovering && previous.IsRecovering() {
		current.Recovering = ""
		current.Annotation.Timestamp = now.Now(ctx)
		current.Annotation.Message = "Leaving recovery mode."
		current.Annotation.User = machineUserName
	}

	// This refers to Swarming's maintenance mode, not machineserver's:
	maintenanceModeMetric := metrics2.GetBoolMetric("machine_processor_device_maintenance", current.Dimensions.AsMetricsTags())
	maintenanceModeMetric.Update(recoveryMessage != "")

	recoveryTimeMetric := metrics2.GetInt64Metric("machine_processor_device_time_in_recovery_mode_s", current.Dimensions.AsMetricsTags())
	if current.IsRecovering() {
		// Report time in recovery mode as a metric.
		recoveryTimeMetric.Update(int64(now.Now(ctx).Sub(current.RecoveryStart).Seconds()))
	} else {
		recoveryTimeMetric.Update(0)
	}

	return current
}

func processChromeOSEvent(ctx context.Context, previous machine.Description, event machine.Event) machine.Description {
	ret := previous.Copy()
	ret.Battery = 0
	ret.Temperature = nil
	ret.DeviceUptime = int32(event.ChromeOS.Uptime / time.Second)

	// ChromeOS doesn't have any conditions that would put it in Recovery mode,
	// and since we made it here we know it's attached.
	ret.Recovering = ""

	// SuppliedDimensions overwrite the existing ones now.
	for k, values := range previous.SuppliedDimensions {
		ret.Dimensions[k] = values
	}
	// Set the ones we know about
	ret.Dimensions[machine.DimOS] = []string{"ChromeOS"}
	ret.Dimensions[machine.DimChromeOSChannel] = []string{event.ChromeOS.Channel}
	ret.Dimensions[machine.DimChromeOSMilestone] = []string{event.ChromeOS.Milestone}
	ret.Dimensions[machine.DimChromeOSReleaseVersion] = []string{event.ChromeOS.ReleaseVersion}

	ret = handleGeneralFields(ctx, ret, event)
	ret = handleRecoveryMode(ctx, previous, ret, ret.Recovering)
	return ret
}

// Based on an incoming iOS event from a test machine, processIOSEvent updates the machine's
// centralized description.
func processIOSEvent(ctx context.Context, previous machine.Description, event machine.Event) machine.Description {
	ret := previous.Copy()

	// The bare "iOS" in the "os" dimension, as well as being useful for
	// filtering in the Swarming UI, is how RebootDevice() tells whether there's
	// an iOS device attached.
	osDimensions := []string{"iOS"}

	// Also add the OS version if it was successfully detected:
	if event.IOS.OSVersion != "" {
		osDimensions = append(osDimensions, "iOS-"+event.IOS.OSVersion)
	}

	ret.Dimensions[machine.DimOS] = osDimensions
	ret.Dimensions[machine.DimDeviceType] = []string{event.IOS.DeviceType}

	maintenanceMessage := ""
	battery := event.IOS.Battery
	if battery != machine.BadBatteryLevel {
		if battery < minBatteryLevel {
			maintenanceMessage += "Battery low. "
		}
		metrics2.GetInt64Metric("machine_processor_device_battery_level", map[string]string{"machine": event.Host.Name}).Update(int64(battery))
	}

	ret = handleGeneralFields(ctx, ret, event)
	ret = handleRecoveryMode(ctx, previous, ret, maintenanceMessage)
	return ret
}

// Based on an incoming iOS event from a test machine, processIOSEvent updates the machine's
// centralized description.
func processPyOCDEvent(ctx context.Context, previous machine.Description, event machine.Event) machine.Description {
	ret := previous.Copy()
	ret.Battery = 0
	ret.Temperature = nil
	ret.Recovering = ""

	ret.Dimensions[machine.DimID] = []string{event.Host.Name}
	ret.Dimensions[machine.DimDeviceType] = []string{event.PyOCD.DeviceType}

	ret = handleGeneralFields(ctx, ret, event)
	return ret
}

// processMissingDeviceEvent processes an event from a machine that expects to have an attached
// device but cannot communicate with it.
func processMissingDeviceEvent(ctx context.Context, previous machine.Description, event machine.Event) machine.Description {
	ret := previous.Copy()
	dimensions := machine.SwarmingDimensions{
		machine.DimID: []string{event.Host.Name},
	}
	// If this machine previously had a connected device and it's no longer present then
	// quarantine the machine.
	//
	// We use the device_type dimension because it is reported for Android and iOS devices
	if len(previous.Dimensions[machine.DimDeviceType]) > 0 {
		ret.Recovering = fmt.Sprintf("Device %q has gone missing", previous.Dimensions[machine.DimDeviceType])
	}
	if previous.SSHUserIP != "" {
		ret.Recovering = fmt.Sprintf("Device %q has gone missing", previous.SSHUserIP)
	}

	ret.Battery = 0
	ret.Temperature = nil
	for k, values := range dimensions {
		ret.Dimensions[k] = values
	}

	ret = handleGeneralFields(ctx, ret, event)
	ret = handleRecoveryMode(ctx, previous, ret, ret.Recovering)
	return ret
}

// processStandaloneEvent processes an event from a machine that is set in the machineserver UI to
// run tests on its own, without an attached device.
func processStandaloneEvent(ctx context.Context, previous machine.Description, event machine.Event) machine.Description {
	ret := previous.Copy()
	ret.Battery = 0
	ret.Temperature = nil
	ret.Dimensions[machine.DimID] = []string{event.Host.Name}
	ret.Dimensions[machine.DimCores] = []string{strconv.Itoa(event.Standalone.Cores)}
	ret.Dimensions[machine.DimOS] = event.Standalone.OSVersions
	ret.Dimensions[machine.DimCPU] = event.Standalone.CPUs
	ret.Dimensions[machine.DimGPU] = event.Standalone.GPUs
	if event.Standalone.IsGCEMachine {
		ret.Dimensions[machine.DimGCE] = []string{"1"}
		ret.Dimensions[machine.DimMachineType] = []string{event.Standalone.GCEMachineType}
	}
	if event.Standalone.IsDockerInstalled {
		ret.Dimensions[machine.DimDockerInstalled] = []string{"true"}
	}
	ret = handleGeneralFields(ctx, ret, event)
	return ret
}

// parseAndroidProperties parses what should be the output of `adb shell
// getprop` and return it as a map[string]string.
func parseAndroidProperties(s string) map[string]string {
	ret := map[string]string{}

	matches := proplines.FindAllStringSubmatch(s, -1)
	for _, line := range matches {
		ret[line[1]] = line[2]
	}
	return ret
}

var (
	// dimensionProperties maps a dimension and its value is the list of all
	// possible device properties that can define that dimension. The
	// product.device should be read (and listed) first, that is, before
	// build.product because the latter is deprecated.
	// https://android.googlesource.com/platform/build/+show/master/tools/buildinfo.sh
	dimensionProperties = map[string][]string{
		"device_os":           {"ro.build.id"},
		"device_os_flavor":    {"ro.product.brand", "ro.product.system.brand"},
		"device_os_type":      {"ro.build.type"},
		machine.DimDeviceType: {"ro.product.device", "ro.build.product", "ro.product.board"},
	}
)

// dimensionsFromAndroidProperties converts dimensions from `adb shell getprop`
// into LUCI machine dimensions.
func dimensionsFromAndroidProperties(prop map[string]string) map[string][]string {
	ret := map[string][]string{}

	for dimName, propNames := range dimensionProperties {
		for _, propName := range propNames {
			if value, ok := prop[propName]; ok {
				if value == "" {
					continue
				}
				arr, ok := ret[dimName]
				if util.In(value, arr) {
					continue
				}
				if ok {
					arr = append(arr, value)
				} else {
					arr = []string{value}
				}
				ret[dimName] = arr
			}
		}
	}

	// Devices such as the Galaxy S7 update build w/o updating android,
	// so we merge them together to catch subtle updates.
	incremental := prop["ro.build.version.incremental"]
	if incremental != "" {
		ret["device_os"] = append(ret["device_os"], fmt.Sprintf("%s_%s", ret["device_os"][0], incremental))
	}

	// Add the first character of each device_os to the dimension.
	osList := append([]string{}, ret["device_os"]...)
	for _, os := range ret["device_os"] {
		firstChar := os[:1]
		if firstChar != "" && strings.ToUpper(firstChar) == firstChar && !util.In(firstChar, osList) {
			osList = append(osList, os[:1])
		}
	}
	sort.Strings(osList)
	if len(osList) > 0 {
		ret["device_os"] = osList
	}

	// Tweaks the 'product.brand' prop to be a little more readable.
	flavors := ret["device_os_flavor"]
	for i, flavor := range flavors {
		flavors[i] = strings.ToLower(flavor)
		if flavors[i] == "aosp" {
			flavors[i] = "android"
		}
	}
	if len(flavors) > 0 {
		ret["device_os_flavor"] = flavors
	}

	if len(ret["device_os"]) > 0 {
		ret["android_devices"] = []string{"1"}
		ret[machine.DimOS] = []string{"Android"}
	}

	// Detects whether the device is running a HWASan build of Android, and if so,
	// advertises it via an extra dimension. See go/skia-android-hwasan and
	// https://developer.android.com/ndk/guides/hwasan.
	if strings.HasSuffix(prop["ro.product.name"], "_hwasan") { // Example: "aosp_sunfish_hwasan".
		ret["android_hwasan_build"] = []string{"1"}
	}

	return ret
}

// Return battery as an integer percentage, i.e. 50% charged returns 50.
//
// The output from dumpsys battery looks like:
//
// Current Battery Service state:
//
//	AC powered: true
//	USB powered: false
//	Wireless powered: false
//	Max charging current: 1500000
//	Max charging voltage: 5000000
//	Charge counter: 2448973
//	status: 2
//	health: 2
//	present: true
//	level: 94
//	scale: 100
//	voltage: 4248
//	temperature: 280
//	technology: Li-ion
func batteryFromAndroidDumpSys(batteryDumpSys string) (int, bool) {
	levelMatch := batteryLevel.FindStringSubmatch(batteryDumpSys)
	if levelMatch == nil {
		sklog.Warningf("Failed to find battery level in %q", batteryDumpSys)
		return machine.BadBatteryLevel, false
	}
	level, err := strconv.Atoi(levelMatch[1])
	if err != nil {
		sklog.Warningf("Failed to convert battery level from %q", levelMatch[1])
		return machine.BadBatteryLevel, false
	}
	scaleMatch := batteryScale.FindStringSubmatch(batteryDumpSys)
	if scaleMatch == nil {
		sklog.Warningf("Failed to find battery scale in %q", batteryDumpSys)
		return machine.BadBatteryLevel, false
	}
	scale, err := strconv.Atoi(scaleMatch[1])
	if err != nil {
		sklog.Warningf("Failed to convert battery scale from %q", scaleMatch[1])
		return machine.BadBatteryLevel, false
	}
	if scale == 0 {
		return machine.BadBatteryLevel, false
	}
	return (100 * level) / scale, true

}

// Return the device temperature as a float. The returned boolean will be false if
// no temperature could be extracted.
//
// The output from `adb shell dumpsys thermalservice` looks like:
//
//	IsStatusOverride: false
//	ThermalEventListeners:
//		callbacks: 1
//		killed: false
//		broadcasts count: -1
//	ThermalStatusListeners:
//		callbacks: 1
//		killed: false
//		broadcasts count: -1
//	Thermal Status: 0
//	Cached temperatures:
//	 Temperature{mValue=-99.9, mType=6, mName=TYPE_POWER_AMPLIFIER, mStatus=0}
//		Temperature{mValue=25.3, mType=4, mName=TYPE_SKIN, mStatus=0}
//		Temperature{mValue=24.0, mType=1, mName=TYPE_CPU, mStatus=0}
//		Temperature{mValue=24.4, mType=3, mName=TYPE_BATTERY, mStatus=0}
//		Temperature{mValue=24.2, mType=5, mName=TYPE_USB_PORT, mStatus=0}
//	HAL Ready: true
//	HAL connection:
//		Sdhms connected: yes
//	Current temperatures from HAL:
//		Temperature{mValue=24.0, mType=1, mName=TYPE_CPU, mStatus=0}
//		Temperature{mValue=24.4, mType=3, mName=TYPE_BATTERY, mStatus=0}
//		Temperature{mValue=25.3, mType=4, mName=TYPE_SKIN, mStatus=0}
//		Temperature{mValue=24.2, mType=5, mName=TYPE_USB_PORT, mStatus=0}
//		Temperature{mValue=-99.9, mType=6, mName=TYPE_POWER_AMPLIFIER, mStatus=0}
//	Current cooling devices from HAL:
//		CoolingDevice{mValue=0, mType=2, mName=TYPE_CPU}
//		CoolingDevice{mValue=0, mType=3, mName=TYPE_GPU}
//		CoolingDevice{mValue=0, mType=1, mName=TYPE_BATTERY}
//		CoolingDevice{mValue=1, mType=4, mName=TYPE_MODEM}
//
// We are only interested in the temperatures found in the 'Current temperatures from HAL'
// section and we return the max of all the temperatures found there.
//
// TODO(jcgregorio) Add support for 'dumpsys hwthermal' which has a different format.
func temperatureFromAndroid(android machine.Android) (map[string]float64, bool) {
	ret := map[string]float64{}
	if len(android.DumpsysThermalService) != 0 {
		// Only use temperatures that appear after "Current temperatures from HAL".
		inCurrentTemperatures := false
		for _, line := range strings.Split(android.DumpsysThermalService, "\n") {
			if !inCurrentTemperatures {
				if strings.HasPrefix(line, "Current temperatures from HAL") {
					inCurrentTemperatures = true
				}
				continue
			}
			// Stop when we get past the Temperature{} lines, i.e. we see a line
			// that doesn't begin with a tab.
			if !strings.HasPrefix(line, "\t") {
				break
			}
			matches := thermalServiceTemperature.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			temp, err := strconv.ParseFloat(matches[1], 64)
			if err != nil {
				continue
			}
			if temp > 1000 {
				// Assume it's in milli-C.
				temp /= 1000
			}
			ret[matches[2]] = temp
		}
	}
	temperatureMatch := batteryTemperature.FindStringSubmatch(android.DumpsysBattery)
	if temperatureMatch != nil {
		// The value is an integer in units of 1/10 C.
		temp10C, err := strconv.Atoi(temperatureMatch[1])
		if err == nil {
			ret[batteryTemperatureKey] = float64(temp10C) / 10
		}
	}
	if len(ret) == 0 {
		return nil, false
	}
	return ret, true
}

func shouldIgnoreTemperature(name string) bool {
	// The "battery_cycle", on some devices, seems to be named and also behave like a
	// non-temperature value. Ignore this value as there are other temperatures
	// to observe - one of which is named "battery".
	return name == "battery_cycle"
}

func findMaxTemperature(temps map[string]float64) float64 {
	if len(temps) == 0 {
		return badTemperature
	}
	max := badTemperature
	for name, temp := range temps {
		if temp > max && !shouldIgnoreTemperature(name) {
			max = temp
		}
	}
	return max
}
