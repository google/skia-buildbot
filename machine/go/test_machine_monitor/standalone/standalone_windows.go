package standalone

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/yusufpapurcu/wmi"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/test_machine_monitor/standalone/crossplatform"
	"go.skia.org/infra/machine/go/test_machine_monitor/standalone/windows"
)

func OSVersions(ctx context.Context) ([]string, error) {
	platform, _, version, err := host.PlatformInformationWithContext(ctx)
	// Return values are like these:
	// "Microsoft Windows Server 2019 Datacenter", "Server", "10.0.17763 Build 17763"
	// "Microsoft Windows 10 Pro", "Standalone Workstation", "10.0.19043 Build 19043"

	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get Windows version")
	}
	return windows.OSVersions(platform, version)
}

func CPUs(ctx context.Context) ([]string, error) {
	cpuStat, err := cpu.Info()
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get CPU info")
	}
	if len(cpuStat) == 0 {
		return nil, skerr.Fmt("cpu.Info() returned 0 entries")
	}

	// As observed by lovisolo@ on 2023-08-22, cpu.Info() returns a single entry on Windows, even if
	// the CPU has multiple cores.
	vendor := cpuStat[0].VendorID
	brandString := cpuStat[0].ModelName

	// For some reason, the CPU model name reported by cpu.Info() on Windows AMD machines contains
	// trailing whitespaces, so we must trim them to prevent crossplatform.CPUs from returning
	// dimensions such as "x86-64-AMD_Ryzen_5_4500U_with_Radeon_Graphics_________".
	if vendor == "AuthenticAMD" {
		brandString = strings.TrimSpace(brandString)
	}

	return crossplatform.CPUs(vendor, brandString)
}

// GPUs returns Swarming-style dimensions representing all GPUs on the host. Each GPU may yield up
// to 3 returned elements: "vendorID", "vendorID:deviceID", and "vendorID:deviceID-driverVersion".
// If no GPUs are found or if the host is running within VMWare, returns ["none"].
func GPUs(ctx context.Context) ([]string, error) {
	var results []windows.GPUQueryResult
	err := wmi.Query("SELECT DriverVersion, PNPDeviceID FROM Win32_VideoController", &results)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to run WMI query to get GPU info")
	}
	return windows.GPUs(results), nil
}

// IsGCEMachine returns true if running on GCE.
func IsGCEMachine() bool {
	return crossplatform.IsGCEMachine()
}

// GCEMachineType returns the GCE machine type.
func GCEMachineType() (string, error) {
	return crossplatform.GCEMachineType()
}

// IsDockerInstalled returns true if Docker is installed.
func IsDockerInstalled(ctx context.Context) bool {
	return crossplatform.IsDockerInstalled(ctx)
}
