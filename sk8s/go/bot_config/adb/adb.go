// Package adb is a simple wrapper around calling adb.
package adb

import (
	"context"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

var (
	// proplines is a regex that matches the output of `adb shell getprop`. Which
	// has output that looks like:
	//
	// [ro.product.manufacturer]: [asus]
	// [ro.product.model]: [Nexus 7]
	// [ro.product.name]: [razor]
	proplines = regexp.MustCompile(`(?m)^\[(?P<key>.+)\]:\s*\[(?P<value>.*)\].*$`)

	// execCommandContext captures exec.CommandContext, which makes testing easier.
	execCommandContext = exec.CommandContext
)

// AdbImpl handles talking to the adb process.
type AdbImpl struct{}

// New returns a new Adb.
func New() AdbImpl {
	return AdbImpl{}
}

// Adb is the interface that AdbImpl provides.
type Adb interface {

	// Properties returns a map[string]string from running "adb shell getprop".
	Properties(ctx context.Context) (map[string]string, error)

	// DimensionsFromProperties mirrors android.py get_dimensions, adding new
	// properties to the passed in 'dim' based on the values it receives from
	// calling Properties.
	//
	// https://cs.chromium.org/chromium/infra/luci/appengine/swarming/swarming_bot/api/platforms/android.py?l=137
	DimensionsFromProperties(ctx context.Context, dim map[string][]string) (map[string][]string, error)
}

// Properties implements the Adb interface.
func (a AdbImpl) Properties(ctx context.Context) (map[string]string, error) {
	ret := map[string]string{}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Note we use execCommandContext, not exec.CommandContext.
	cmd := execCommandContext(ctx, "adb", "shell", "getprop")

	b, err := cmd.CombinedOutput()
	if err != nil {
		return nil, skerr.Wrapf(err, "Err: %q", string(b))
	}

	matches := proplines.FindAllStringSubmatch(string(b), -1)
	for _, line := range matches {
		ret[line[1]] = line[2]
	}

	return ret, nil
}

var (
	// dimensionProperties maps a dimension and its value is the list of all
	// possible device properties that can define that dimension. The
	// product.device should be read (and listed) first, that is, before
	// build.product because the latter is deprecated.
	// https://android.googlesource.com/platform/build/+/master/tools/buildinfo.sh
	dimensionProperties = map[string][]string{
		"device_os":        {"ro.build.id"},
		"device_os_flavor": {"ro.product.brand", "ro.product.system.brand"},
		"device_os_type":   {"ro.build.type"},
		"device_type":      {"ro.product.device", "ro.build.product", "ro.product.board"},
	}
)

// DimensionsFromProperties implements the Adb interface.
func (a AdbImpl) DimensionsFromProperties(ctx context.Context, dim map[string][]string) (map[string][]string, error) {
	prop, err := a.Properties(ctx)
	if err != nil {
		return dim, skerr.Wrapf(err, "Failed to get properties.")
	}
	for dimName, propNames := range dimensionProperties {
		for _, propName := range propNames {
			if value, ok := prop[propName]; ok {
				arr, ok := dim[dimName]
				if util.In(value, arr) {
					continue
				}
				if ok {
					arr = append(arr, value)
				} else {
					arr = []string{value}
				}
				dim[dimName] = arr
			}
		}
	}

	// Add the first character of each device_os to the dimension.
	osList := append([]string{}, dim["device_os"]...)
	for _, os := range dim["device_os"] {
		if os[:1] != "" && strings.ToUpper(os[:1]) == os[:1] {
			osList = append(osList, os[:1])
		}
	}
	sort.Strings(osList)
	if len(osList) > 0 {
		dim["device_os"] = osList
	}

	// Tweaks the 'product.brand' prop to be a little more readable.
	flavors := dim["device_os_flavor"]
	for i, flavor := range flavors {
		flavors[i] = strings.ToLower(flavor)
		if flavors[i] == "aosp" {
			flavors[i] = "android"
		}
	}
	if len(flavors) > 0 {
		dim["device_os_flavor"] = flavors
	}

	dim["android_devices"] = []string{"1"}
	dim["os"] = []string{"Android"}

	return dim, nil
}

// Assert that AdbImpl implements the Adb interface.
var _ Adb = AdbImpl{}
