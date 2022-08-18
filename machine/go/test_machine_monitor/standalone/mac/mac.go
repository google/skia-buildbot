// Package mac contains Mac-specific pieces of interrogation which are nonetheless testable on
// arbitrary platforms.
package mac

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/test_machine_monitor/standalone/gputable"
	"howett.net/plist"
)

// VersionsOfAllPrecisions splits a macOS version like 1.2.3 into an array of versions of all
// precisions, like ["Mac", "Mac-1", "Mac-1.2", "Mac-1.2.3"].
func VersionsOfAllPrecisions(version string) []string {
	subversions := strings.Split(version, ".")
	ret := []string{"Mac", "Mac-" + subversions[0]}
	for i, subversion := range subversions[1:] {
		ret = append(ret, ret[i+1]+"."+subversion)
	}
	return ret
}

// intelModel extracts the CPU model name from its display name and returns it. For example, it
// pulls "i7-9750H" out of "Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz". Returns "" if extraction
// fails.
func intelModel(brandString string) string {
	regexes := []*regexp.Regexp{
		regexp.MustCompile(` ([a-zA-Z]\d-\d{4}[A-Z]{0,2} [vV]\d) `),
		regexp.MustCompile(` ([a-zA-Z]\d-\d{4}[A-Z]{0,2}) `),
		regexp.MustCompile(` ([A-Z]\d{4}[A-Z]{0,2}) `),
		regexp.MustCompile(` ((:?[A-Z][a-z]+ )+GCE)`),
	}
	for _, regex := range regexes {
		if matches := regex.FindStringSubmatch(brandString); len(matches) >= 2 {
			return matches[1]
		}
	}
	return ""
}

// CPUs is the brains behind the CPUs() function, broken off for testing.
func CPUs(arch string, vendor string, brandString string) ([]string, error) {
	// gopsutil can spit out i386, i686, arm, aarch64, ia64, or x86_64 under Windows; "" as a
	// fallback; whatever uname's utsname.machine struct member (a string) has on POSIX. On our lab
	// Macs, that's arm64 or x64_86 (exposed by uname -m). Our lab Linuxes: x86_64.
	arch_to_isa := map[string]string{
		"x86_64":  "x86",
		"amd64":   "x86",
		"i386":    "x86",
		"i686":    "x86",
		"aarch64": "arm64",
		"arm64":   "arm64",
	}
	isa, ok := arch_to_isa[arch]
	if !ok {
		return nil, skerr.Fmt("host had unknown architecture: %s", arch)
	}

	var bitness string
	// Fall back to int-size test if the width isn't explicit in the arch name:
	if strings.HasSuffix(arch, "64") || strconv.IntSize > 1<<32 {
		bitness = "64"
	} else {
		bitness = "32"
	}
	ret := []string{isa, fmt.Sprintf("%s-%s", isa, bitness)}

	var model string
	if vendor == "GenuineIntel" {
		model = intelModel(brandString)
	} else {
		model = strings.ReplaceAll(brandString, " ", "_")
	}
	if len(model) > 0 {
		ret = append(ret, fmt.Sprintf("%s-%s-%s", isa, bitness, model))
	}

	return ret, nil
}

type GPU struct {
	// Any string field missing from the plist ends up "".
	ID       string `plist:"spdisplays_device-id"`
	VendorID string `plist:"spdisplays_vendor-id"`
	Vendor   string `plist:"spdisplays_vendor"`
	Version  string `plist:"spdisplays_gmux-version"`
	Model    string `plist:"sppci_model"`
}

// GPUsFromSystemProfilerXML returns the GPUs found on a Mac by System Profiler, as expressed
// through its -xml output option. If the output doesn't look valid, returns an error and no GPUs.
// If there are no GPUs, will return 0 GPUs even if no error occurs.
func GPUsFromSystemProfilerXML(xml string) ([]GPU, error) {
	type OuterDict struct {
		GPUs []GPU `plist:"_items"`
	}
	var profilerOutput []OuterDict

	_, err := plist.Unmarshal([]byte(xml), &profilerOutput)

	if err != nil {
		return nil, skerr.Wrapf(err, "couldn't unmarshal XML from System Profiler")
	}
	if len(profilerOutput) < 1 {
		return nil, skerr.Fmt("System Profiler returned no info")
	}
	return profilerOutput[0].GPUs, nil
}

// DimensionsFromGPUs turns a slice of Mac GPUs into Swarming-style dimensions, e.g. ["Intel
// (8086)", "Intel Coffee Lake H UHD Graphics 630 (8086:3e9b)"]. If there are no GPUs, return
// ["none"].
func DimensionsFromGPUs(gpus []GPU) []string {
	var dimensions []string
	for _, gpu := range gpus {
		if gpu.ID == "" {
			continue
		}
		gpuID := strings.TrimPrefix(gpu.ID, "0x")
		var vendorID gputable.VendorID
		if gpu.VendorID != "" {
			// NVidia
			vendorID = gputable.VendorID(strings.TrimPrefix(gpu.VendorID, "0x"))
		} else if gpu.Vendor != "" {
			// Intel and ATI
			re := regexp.MustCompile(`\(0x([0-9a-f]{4})\)`)
			if matches := re.FindStringSubmatch(gpu.Vendor); matches != nil {
				vendorID = gputable.VendorID(matches[1])
			}
		}

		vendorName := ""
		if gpu.Model != "" {
			ok := false
			// The first word is pretty much always the company name:
			vendorName, _, ok = strings.Cut(gpu.Model, " ")
			if !ok {
				// This has apparently never happened in the history of Swarming's introspection
				// code, because it would have raised an uncaught exception there:
				sklog.Warningf("GPU device name was only a single word: %s", gpu.Model)
				continue
			}
		}

		// macOS 10.13 stopped including the vendor ID in the spdisplays_vendor string. Infer it
		// from the vendor name instead.
		if vendorID == "" {
			vendorID = gputable.VendorNameToID(vendorName)
		}
		if vendorID == "" && gpu.Vendor != "" {
			re := regexp.MustCompile(`sppci_vendor_([a-z]+)$`)
			if matches := re.FindStringSubmatch(gpu.Vendor); matches != nil {
				vendorName = matches[1]
				vendorID = gputable.VendorNameToID(vendorName)
			}
		}
		if vendorID == "" {
			vendorID = gputable.VendorID("UNKNOWN")
		} else if vendorID == "15ad" {
			// This is VMWare, which we consider as not having any GPUs.
			return []string{"none"}
		}

		dimensions = append(dimensions, string(vendorID), fmt.Sprintf("%s:%s", vendorID, gpuID))

		// Add GPU version:
		if gpu.Version != "" {
			re := regexp.MustCompile(`([0-9.]+) \[([0-9.]+)\]`)
			if matches := re.FindStringSubmatch(gpu.Version); matches != nil {
				dimensions = append(dimensions, fmt.Sprintf("%s:%s-%s-%s", vendorID, gpuID, matches[1], matches[2]))
			}
		}
	}
	if len(dimensions) == 0 {
		dimensions = []string{"none"}
	}
	sort.Strings(dimensions)
	return dimensions
}
