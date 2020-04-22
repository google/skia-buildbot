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

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine"
)

const (
	badBatteryLevel = -99

	// minBatteryLevel is the minimum percentage that a battery needs to be
	// charged before testing is allowed. Below this the device should be
	// quarantined.
	minBatteryLevel = 30
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
	batteryLevel = regexp.MustCompile(`(?m)level:\s+(\d+)\n`)

	// batteryScale is a regex that matches the output of `adb shell dumpsys battery` and looks
	// for the battery scale which looks like:
	//
	//    scale: 100
	batteryScale = regexp.MustCompile(`(?m)scale:\s+(\d+)\n`)
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
	machineID := event.Host.Name
	dimensions := dimensionsFromAndroidProperties(parseAndroidProperties(event.Android.GetProp))
	dimensions[machine.DimID] = []string{machineID}

	// TODO(jcgregorio) Come up with a better test than this, maybe sent info
	// from back in machine.Event?
	if strings.HasPrefix(dimensions[machine.DimID][0], "skia-rpi2-") {
		dimensions["inside_docker"] = []string{"1", "containerd"}
	}

	battery, ok := batteryFromAndroidDumpSys(event.Android.DumpsysBattery)
	if ok {
		if battery < minBatteryLevel {
			dimensions[machine.DimQuarantined] = []string{fmt.Sprintf("Battery is too low: %d < %d (%%)", battery, minBatteryLevel)}
		}
		metrics2.GetInt64Metric("machine_processor_device_battery_level", map[string]string{"machine": machineID}).Update(int64(battery))
	}

	// If this machine previously had a connected device and it's no longer
	// present then quarantine the machine.
	//
	// We use the device_type dimension because it is reported for both Android
	// and iOS devices.
	if len(previous.Dimensions[machine.DimDeviceType]) > 0 && len(dimensions[machine.DimDeviceType]) == 0 {
		dimensions[machine.DimQuarantined] = []string{fmt.Sprintf("Device %q has gone missing", previous.Dimensions[machine.DimDeviceType])}
	}

	// Quarantine devices in maintenance mode.
	if previous.Mode == machine.ModeMaintenance && len(previous.Dimensions[machine.DimQuarantined]) == 0 {
		dimensions[machine.DimQuarantined] = []string{"Device is quarantined for maintenance"}
	}

	ret := previous.Copy()
	ret.Battery = battery
	for k, values := range dimensions {
		ret.Dimensions[k] = values
	}

	// If the machine was quarantined, but hasn't been quarantined this trip
	// through Process then take the machine out of quarantine.
	if previous.Mode == machine.ModeAvailable && len(previous.Dimensions[machine.DimQuarantined]) != 0 && len(dimensions[machine.DimQuarantined]) == 0 {
		delete(ret.Dimensions, machine.DimQuarantined)
	}

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
	// https://android.googlesource.com/platform/build/+/master/tools/buildinfo.sh
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

	// Add the first character of each device_os to the dimension.
	osList := append([]string{}, ret["device_os"]...)
	for _, os := range ret["device_os"] {
		if os[:1] != "" && strings.ToUpper(os[:1]) == os[:1] {
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

	return ret
}

// Return battery as an integer percentage, i.e. 50% charged returns 50.
//
// The output from dumpsys battery looks like:
//
// Current Battery Service state:
//  AC powered: true
//  USB powered: false
//  Wireless powered: false
//  Max charging current: 1500000
//  Max charging voltage: 5000000
//  Charge counter: 2448973
//  status: 2
//  health: 2
//  present: true
//  level: 94
//  scale: 100
//  voltage: 4248
//  temperature: 280
//  technology: Li-ion

func batteryFromAndroidDumpSys(batteryDumpSys string) (int, bool) {
	levelMatch := batteryLevel.FindStringSubmatch(batteryDumpSys)
	if levelMatch == nil {
		return badBatteryLevel, false
	}
	level, err := strconv.Atoi(levelMatch[1])
	if err != nil {
		return badBatteryLevel, false
	}
	scaleMatch := batteryScale.FindStringSubmatch(batteryDumpSys)
	if scaleMatch == nil {
		return badBatteryLevel, false
	}
	scale, err := strconv.Atoi(scaleMatch[1])
	if err != nil {
		return badBatteryLevel, false
	}
	if scale == 0 {
		return badBatteryLevel, false
	}
	return (100 * level) / scale, true

}
