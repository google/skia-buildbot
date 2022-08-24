package standalone

import (
	"context"
	"os"

	"github.com/shirou/gopsutil/host"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/test_machine_monitor/standalone/crossplatform"
	"go.skia.org/infra/machine/go/test_machine_monitor/standalone/linux"
)

func OSVersions(ctx context.Context) ([]string, error) {
	platform, _, version, err := host.PlatformInformationWithContext(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get Linux version")
	}
	return linux.OSVersions(platform, version), nil
}

// CPUs returns a Swarming-style description of the host's CPU, in various precisions.
//
// Swarming goes to special trouble on Linux to return "32" if running a 32-bit userland on a 64-bit
// kernel, we do not. None of our jobs care about that distinction, nor, I think, do any of our
// boxes run like that.
func CPUs(ctx context.Context) ([]string, error) {
	arch, err := host.KernelArch()
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get Linux CPU architecture")
	}

	procFile, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer procFile.Close()

	vendor, brandString, err := linux.VendorAndBrand(procFile)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get vendor and brand string")
	}

	return crossplatform.CPUs(arch, vendor, brandString)
}

func GPUs(ctx context.Context) ([]string, error) {
	var ret []string
	return ret, nil
}
