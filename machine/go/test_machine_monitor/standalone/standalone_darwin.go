package standalone

import (
	"context"

	"github.com/shirou/gopsutil/host"
	"go.skia.org/infra/go/skerr"
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
	vendor, err := unix.Sysctl("machdep.cpu.vendor")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	brandString, err := unix.Sysctl("machdep.cpu.brand_string")
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return cpusCore(arch, vendor, brandString)
}
