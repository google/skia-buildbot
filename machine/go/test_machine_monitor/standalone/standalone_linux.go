package standalone

import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/host"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/common"
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

// CPUs returns a Swarming-style description of the host's CPU, in all available precisions.
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

// nvidiaVersion returns the version of the installed Nvidia GPU driver, "" if not available.
func nvidiaDriverVersion() string {
	contents, err := os.ReadFile("/sys/module/nvidia/version")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(contents))
}

var dpkgVersionRegex = regexp.MustCompile(`(?m)^Version: (\d+\.\d+\.\d+)`)

// intelDriverVersion returns the version of the installed Intel GPU driver, "" if not available.
func intelDriverVersion(ctx context.Context) string {
	status, err := common.TrimmedCommandOutput(ctx, "dpkg", "-s", "libgl1-mesa-dri")
	if err != nil {
		return ""
	}
	groups := dpkgVersionRegex.FindStringSubmatch(status)
	if groups == nil {
		return ""
	}
	return groups[1]
}

// GPUs returns a Swarming-style description of all the host's GPUs, in all available precisions,
// drawn from the lspci commandline util. If lspci is absent, returns an error.
func GPUs(ctx context.Context) ([]string, error) {
	lspciOutput, err := common.TrimmedCommandOutput(ctx, "lspci", "-mm", "-nn")
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to run lspci to get GPU info. Output was '%s'", lspciOutput)
	}
	return linux.GPUs(ctx, lspciOutput, nvidiaDriverVersion, intelDriverVersion)
}
