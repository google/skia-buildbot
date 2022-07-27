package standalone

import (
	"context"

	"github.com/shirou/gopsutil/host"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/common"
	"golang.org/x/sys/unix"
)

// OSVersions returns the macOS version in all possible precisions. For example, 10.5.7 would yield
// ["Mac-10", "Mac-10.5", "Mac-10.5.7"].
func OSVersions(ctx context.Context) ([]string, error) {
	_, _, platformVersion, err := host.PlatformInformation()
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get macOS version")
	}
	return macVersionsOfAllPrecisions(platformVersion), nil
}

// CPUs returns a Swarming-style description of the host's CPU, in various precisions, e.g. ["x86",
// "x86-64", "x86-64-i5-5350U"]. The first (ISA) and second (bit width) will always be returned (if
// returned error is nil). The third (model number) will be added if we succeed in extracting it.
//
// Swarming goes to special trouble on Linux to return "32" if running a 32-bit userland on a 64-bit
// kernel, we do not. None of our jobs care about that distinction, nor, I think, do any of our
// boxes run like that.
func CPUs(ctx context.Context) ([]string, error) {
	arch, err := host.KernelArch()
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get Mac CPU architecture")
	}
	// It is perfectly normal for these sysctl keys to be missing sometimes:
	vendor, _ := unix.Sysctl("machdep.cpu.vendor") // Sysctl returns "" on failure.
	brandString, _ := unix.Sysctl("machdep.cpu.brand_string")
	return macCPUs(arch, vendor, brandString)
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

	gpus, err := gpusFromSystemProfilerXML(xml)
	if err != nil {
		return nil, skerr.Wrapf(err, "couldn't get GPUs from System Profiler XML")
	}
	return dimensionsFromMacGPUs(gpus), nil
}
