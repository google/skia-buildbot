// Package ios exposes routines for communicating with locally attached iOS devices via
// libidevicemobile CLI tools.
package ios

import (
	"context"
	"strconv"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/common"
	"go.skia.org/infra/machine/go/machine"
)

type IOS interface {
	Reboot(ctx context.Context) error
	OSVersion(ctx context.Context) (string, error)
	DeviceType(ctx context.Context) (string, error)
	BatteryLevel(ctx context.Context) (int, error)
}

// We began attaching all our methods to an interface/struct only so we could mock the whole thing
// out. This was before we could mock out CLI invocations using Contexts.
type IOSImpl struct{}

func New() IOSImpl {
	return IOSImpl{}
}

// Reboot restarts an arbitrary attached iOS device. (We never have more than one attached in
// practice.)
func (ios IOSImpl) Reboot(ctx context.Context) error {
	_, err := common.TrimmedCommandOutput(ctx, "idevicediagnostics", "restart")
	return skerr.Wrapf(err, "Failed to restart iOS device")
}

// OSVersion returns the version of iOS (or iPadOS, etc.) running on the attached device, e.g.
// "13.3.1".
func (ios IOSImpl) OSVersion(ctx context.Context) (string, error) {
	output, err := common.TrimmedCommandOutput(ctx, "ideviceinfo", "-k", "ProductVersion")
	return output, skerr.Wrapf(err, "Failed to get iOS device OS version. Output was '%s'", output)
}

// DeviceType returns the Apple model identifier of the attached iDevice, e.g. "iPhone10,1".
func (ios IOSImpl) DeviceType(ctx context.Context) (string, error) {
	output, err := common.TrimmedCommandOutput(ctx, "ideviceinfo", "-k", "ProductType")
	return output, skerr.Wrapf(err, "Failed to get iOS device type. Output was '%s'", output)
}

// BatteryLevel returns the battery-full percentage of the attached device, or BadBatteryLevel if an
// error occurs.
func (ios IOSImpl) BatteryLevel(ctx context.Context) (int, error) {
	battery_level := machine.BadBatteryLevel
	output, err := common.TrimmedCommandOutput(ctx, "ideviceinfo", "--domain", "com.apple.mobile.battery", "-k", "BatteryCurrentCapacity")
	if err == nil {
		var int_output int
		if int_output, err = strconv.Atoi(output); err == nil {
			battery_level = int_output
		}
	}
	return battery_level, skerr.Wrapf(err, "Failed to get iOS battery level. Output was '%s'", output)
}

var _ IOS = IOSImpl{}
