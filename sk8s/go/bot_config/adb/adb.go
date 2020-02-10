// Package adb is a simple wrapper around calling adb.
package adb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/util"
)

var (
	// proplines is a regex that matches the output of adb shell getprop across
	// multiple lines. The output looks like:
	//
	// [ro.product.manufacturer]: [asus]
	// [ro.product.model]: [Nexus 7]
	// [ro.product.name]: [razor]
	proplines = regexp.MustCompile(`(?m)^\[(?P<key>.+)\]:\s*\[(?P<value>.*)\].*$`)
)

// Properties returns a map[string]string from running "adb shell getprop".
func Properties(ctx context.Context) (map[string]string, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	ret := map[string]string{}

	cmd := exec.Command{
		Name:    "adb",
		Args:    []string{"shell", "getprop"},
		Stdout:  stdout,
		Stderr:  stderr,
		Timeout: 5 * time.Second,
	}
	if err := exec.Run(ctx, &cmd); err != nil {
		return nil, fmt.Errorf("Failed to run adb shell getprop %q: %s", stderr.String(), err)
	}
	lines := stdout.String()
	matches := proplines.FindAllStringSubmatch(lines, -1)
	for _, line := range matches {
		ret[line[1]] = line[2]
	}

	return ret, nil
}

var (
	// packageVersionRe is a regex that matches the output of adb shell dumpsys package
	// for the versionName, which looks like:
	//
	// versionName=8.1.86 (2287566-436)
	packageVersionRe = regexp.MustCompile(`(?m)^\s*versionName=(?P<name>[^\s]+)`)
)

// packageVersion return the version of an installed package in a string slice,
// returning an empty slice if the command fails.
func packageVersion(ctx context.Context, errout io.Writer, pkg string) []string {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := exec.Command{
		Name:    "adb",
		Args:    []string{"shell", "dumpsys", "package", pkg},
		Stdout:  stdout,
		Stderr:  stderr,
		Timeout: 5 * time.Second,
	}
	if err := exec.Run(ctx, &cmd); err != nil {
		_, _ = fmt.Fprintf(errout, "Error: Failed to run adb dumpsys package %q: %s", stderr.String(), err)
	}
	versionMatch := packageVersionRe.FindAllStringSubmatch(stdout.String(), 1)
	if len(versionMatch) != 1 {
		return []string{}
	}
	return []string{versionMatch[0][1]}
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

// DimensionsFromProperties tries to match android.py get_dimensions.
//
// https://cs.chromium.org/chromium/infra/luci/appengine/swarming/swarming_bot/api/platforms/android.py?l=137
func DimensionsFromProperties(ctx context.Context, errout io.Writer, dim map[string][]string) map[string][]string {
	prop, err := Properties(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(errout, "Error: Failed to get properties: %s\n", err)
		return dim
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
	os_list := append([]string{}, dim["device_os"]...)
	for _, os := range dim["device_os"] {
		if os[:1] != "" && strings.ToUpper(os[:1]) == os[:1] {
			os_list = append(os_list, os[:1])
		}
	}
	sort.Strings(os_list)
	dim["device_os"] = os_list

	// Tweaks the 'product.brand' prop to be a little more readable.
	flavors := dim["device_os_flavor"]
	for i, flavor := range flavors {
		flavors[i] = strings.ToLower(flavor)
		if flavors[i] == "aosp" {
			flavors[i] = "android"
		}
	}
	dim["device_os_flavor"] = flavors

	dim["android_devices"] = []string{"1"}
	dim["os"] = []string{"Android"}

	return dim
}
