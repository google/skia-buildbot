// Package common contains interrogation-related code common to multiple platforms.
package crossplatform

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/shirou/gopsutil/host"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
)

// isaAndBitness, given an architecture like "x86_64", extracts both an instruction set architecture
// and bit width, e.g. "x86" and "64".
//
// Swarming counterparts:
// - https://chromium.googlesource.com/infra/luci/luci-py/+/c3bea95091caef1800d102bae28dfc715a4043bc/appengine/swarming/swarming_bot/api/os_utilities.py#228
// - https://chromium.googlesource.com/infra/luci/luci-py/+/c3bea95091caef1800d102bae28dfc715a4043bc/appengine/swarming/swarming_bot/api/os_utilities.py#250
func isaAndBitness(arch string) (isa, bitness string, err error) {
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
		return "", "", skerr.Fmt("host had unknown architecture: %s", arch)
	}

	// Fall back to int-size test if the width isn't explicit in the arch name:
	if strings.HasSuffix(arch, "64") || strconv.IntSize > 1<<32 {
		bitness = "64"
	} else {
		bitness = "32"
	}

	return
}

// intelModel extracts the CPU model name from its display name and returns it. For example, it
// pulls "i7-9750H" out of "Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz". Returns "" if extraction
// fails.
//
// Swarming counterpart:
// https://chromium.googlesource.com/infra/luci/luci-py/+/c3bea95091caef1800d102bae28dfc715a4043bc/appengine/swarming/swarming_bot/api/os_utilities.py#301
func intelModel(brandString string) string {
	regexes := []*regexp.Regexp{
		regexp.MustCompile(` ([a-zA-Z]\d-\d{4}[A-Z]{0,2} [vV]\d) `),
		regexp.MustCompile(` ([a-zA-Z]\d-\d{4}[A-Z]{0,2}) `),
		regexp.MustCompile(` ([a-zA-Z]\d-\d{4}[A-Z]\d) `),
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

// cpuModel takes a vendor name and a brand string (see CPUs()) and synthesizes a descriptive model
// string from the two. If it fails to do so, it returns "".
func cpuModel(vendor, brandString string) string {
	if vendor == "GenuineIntel" {
		return intelModel(brandString)
	}
	return brandString
}

// CPUs is the brains behind various platform-specific CPUs() functions, broken off for testing. It
// takes the vendor name (e.g. "GenuineIntel", "AuthenticAMD") and a "brand string", which is a
// model signifier whose format is vendor-specific (e.g. "Intel(R) Xeon(R) CPU @ 2.00GHz", "AMD
// EPYC 7B12"), and returns a Swarming-style description of the host's CPU, in various precisions,
// e.g. ["x86", "x86-64", "x86-64-i5-5350U"]. The first (ISA) and second (bit width) will always be
// returned (if returned error is nil). The third (model number) will be added if we succeed in
// extracting it.
//
// Swarming counterpart:
// https://chromium.googlesource.com/infra/luci/luci-py/+/c3bea95091caef1800d102bae28dfc715a4043bc/appengine/swarming/swarming_bot/api/os_utilities.py#323
func CPUs(vendor, brandString string) ([]string, error) {
	arch, err := hostKernelArch() // Example: "x86_64".
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get CPU architecture")
	}

	isa, bitness, err := isaAndBitness(arch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	ret := []string{isa, fmt.Sprintf("%s-%s", isa, bitness)}

	// Normalize the CPU name reported by GCE using the same algorithm as Swarming for backwards
	// compatibility. See
	// https://chromium.googlesource.com/infra/luci/luci-py/+/c3bea95091caef1800d102bae28dfc715a4043bc/appengine/swarming/swarming_bot/api/platforms/gce.py#327.
	if IsGCEMachine() {
		gceCPUPlatform, err := metadataGet("instance/cpu-platform")
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		if strings.HasPrefix(gceCPUPlatform, "Intel ") { // Example: "Intel Haswell".
			vendor = "GenuineIntel"
			brandString = fmt.Sprintf("Intel(R) Xeon(R) CPU %s GCE", strings.TrimPrefix(gceCPUPlatform, "Intel "))
		} else if strings.HasPrefix(gceCPUPlatform, "AMD ") { // Example: "AMD Rome".
			vendor = "AuthenticAMD"
			brandString = gceCPUPlatform + " GCE"
		}
	}

	model := cpuModel(vendor, brandString)
	if model != "" {
		ret = append(ret, fmt.Sprintf("%s-%s-%s", isa, bitness, strings.ReplaceAll(model, " ", "_")))
	}

	return ret, nil
}

// VersionsOfAllPrecisions splits a version like 1.2.3 into an array of versions of all precisions
// and prepends a constant prefix, resulting in a string slice like {"Mac", "Mac-1", "Mac-1.2",
// "Mac-1.2.3"}. If version is empty, return only the prefix, like {"Mac"}.
func VersionsOfAllPrecisions(prefix, version string) []string {
	ret := []string{prefix}
	subversions := strings.Split(version, ".")
	if version != "" {
		ret = append(ret, prefix+"-"+subversions[0])
	}
	for i, subversion := range subversions[1:] {
		ret = append(ret, ret[i+1]+"."+subversion)
	}
	return ret
}

// IsGCEMachine returns true if running on GCE.
func IsGCEMachine() bool {
	return metadataOnGCE() // Cached internally by the metadata module.
}

var cachedMachineType = ""

// GCEMachineType returns the machine type reported by the metadata server. Returns the empty
// string if not running on GCE.
//
// Inspired by
// https://chromium.googlesource.com/infra/luci/luci-py/+/84efecd73da77529df8a3fb6e37d232a068e6312/appengine/swarming/swarming_bot/api/platforms/gce.py#90.
func GCEMachineType() (string, error) {
	if !IsGCEMachine() {
		return "", nil
	}

	if cachedMachineType == "" {
		machineType, err := metadataGet("instance/machine-type")
		if err != nil {
			return "", skerr.Wrapf(err, "failed to get the machine type from the metadata server")
		}
		cachedMachineType = machineType[strings.LastIndex(machineType, "/")+1:]
	}

	return cachedMachineType, nil
}

// IsDockerInstalled returns true if Docker is installed.
func IsDockerInstalled(ctx context.Context) bool {
	_, err := exec.RunSimple(ctx, "docker version")
	return err == nil
}

// We overwrite these aliases from tests.
var (
	hostKernelArch = host.KernelArch
	metadataOnGCE  = metadata.OnGCE
	metadataGet    = metadata.Get
)
