package standalone

import (
	"context"

	"github.com/shirou/gopsutil/host"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/common"
	"go.skia.org/infra/machine/go/test_machine_monitor/standalone/crossplatform"
	"go.skia.org/infra/machine/go/test_machine_monitor/standalone/mac"
	"golang.org/x/sys/unix"
)

// OSVersions returns the macOS version in all possible precisions. For example, 10.5.7 would yield
// ["Mac-10", "Mac-10.5", "Mac-10.5.7"].
func OSVersions(ctx context.Context) ([]string, error) {
	_, _, platformVersion, err := host.PlatformInformation()
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get macOS version")
	}
	return mac.VersionsOfAllPrecisions(platformVersion), nil
}

// CPUs returns a Swarming-style description of the host's CPU, in various precisions.
func CPUs(ctx context.Context) ([]string, error) {
	arch, err := host.KernelArch()
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get Mac CPU architecture")
	}
	// It is perfectly normal for these sysctl keys to be missing sometimes:
	vendor, _ := unix.Sysctl("machdep.cpu.vendor") // Sysctl returns "" on failure.
	brandString, _ := unix.Sysctl("machdep.cpu.brand_string")
	return crossplatform.CPUs(arch, vendor, brandString)
}

// GPUs returns Swarming-style descriptions of all the host's GPUs, in various precisions, all
// flattened into a single array, e.g. ["Intel (8086)", "Intel Broadwell HD Graphics 6000
// (8086:1626)", "Intel (8086)", "8086:9a49", "8086:9a49-22.0.5"]. At most, an array element may
// have 4 elements of precision: vendor ID, vendor name, device ID, and device name (in that order).
// However, the formats of these are device- and OS-dependent.
func GPUs(ctx context.Context) ([]string, error) {
	xml, err := common.TrimmedCommandOutput(ctx, "system_profiler", "SPDisplaysDataType", "-xml")
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to run System Profiler to get GPU info. Output was '%s'", xml)
	}

	gpus, err := mac.GPUsFromSystemProfilerXML(xml)
	if err != nil {
		return nil, skerr.Wrapf(err, "couldn't get GPUs from System Profiler XML")
	}
	return mac.DimensionsFromGPUs(gpus), nil
}
