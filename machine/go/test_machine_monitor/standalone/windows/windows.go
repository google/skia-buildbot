// Package windows contains Windows-specific pieces of interrogation which are nonetheless testable
// on arbitrary platforms.
package windows

import (
	"regexp"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/test_machine_monitor/standalone/gputable"
)

var platformRegex = regexp.MustCompile(`Microsoft Windows ([^ ]+)`)

// version is built using the format string "%d.%d.%d Build %d", where the last 2 %d
// tokens are filled identically.
var versionRegex = regexp.MustCompile(`\d+\.\d+\.(\d+) `)

func OSVersions(platform, version string) ([]string, error) {
	groups := platformRegex.FindStringSubmatch(platform)
	if groups == nil {
		return nil, skerr.Fmt("couldn't parse Windows platform string %q", platform)
	}
	major := groups[1] // Possible values include 8.1, 10, XP, and Server.

	groups = versionRegex.FindStringSubmatch(version)
	if groups == nil {
		return nil, skerr.Fmt("couldn't parse Windows version string %q", version)
	}
	build := groups[1]

	// We don't detect service pack numbers at the moment, which are a pre-Windows-10 phenomenon. If
	// we wanted to, we could extract them from the end of platform or do our own registry query for
	// CSDVersion.

	return []string{"Windows", "Windows-" + major, "Windows-" + major + "-" + build}, nil
}

type GPUQueryResult struct {
	DriverVersion string
	PNPDeviceID   string
}

var gpuVendorRegex = regexp.MustCompile(`VEN_([0-9A-F]{4})`)
var gpuDeviceRegex = regexp.MustCompile(`DEV_([0-9A-F]{4})`)

func GPUs(results []GPUQueryResult) []string {
	// Extract the first group of the regex from a raw device ID, returning "UNKNOWN" on failure.
	extract := func(regex *regexp.Regexp, rawDeviceID string) string {
		groups := regex.FindStringSubmatch(rawDeviceID)
		if groups != nil {
			return strings.ToLower(groups[1])
		}
		return "UNKNOWN"
	}

	var dimensions []string
	for _, result := range results {
		vendorID := extract(gpuVendorRegex, result.PNPDeviceID)
		deviceID := extract(gpuDeviceRegex, result.PNPDeviceID)

		if vendorID == gputable.VMWare {
			return []string{"none"}
		}
		dimensions = append(dimensions, vendorID, vendorID+":"+deviceID)
		if result.DriverVersion != "" {
			dimensions = append(dimensions, vendorID+":"+deviceID+"-"+result.DriverVersion)
		}
	}
	if len(dimensions) == 0 {
		return []string{"none"}
	}
	return dimensions
}
