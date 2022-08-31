package standalone

import (
	"context"

	"github.com/shirou/gopsutil/host"
	"go.skia.org/infra/go/skerr"
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
	var ret []string
	return ret, nil
}

func GPUs(ctx context.Context) ([]string, error) {
	var ret []string
	return ret, nil
}
