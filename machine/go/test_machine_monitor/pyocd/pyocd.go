// Package pyocd exposes routines for communicating with locally attached devices via
// the pyocd CLI tool.
package pyocd

import (
	"context"
	"os"
	"strings"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/common"
)

type PyOCD interface {
	DeviceType(ctx context.Context) (string, error)
}

const configPath = `/etc/skia/pyocd_config.json`

type pyocdConfig struct {
	DeviceType string `json:"device_type"`
}

// We began attaching all our methods to an interface/struct only so we could mock the whole thing
// out. This was before we could mock out CLI invocations using Contexts.
type pyocdImpl struct {
	deviceType string
}

func New() pyocdImpl {
	p := pyocdImpl{}
	cfgBytes, err := os.ReadFile(configPath)
	if err != nil {
		sklog.Infof("Did not detect PyOCD configuration %s", err)
	} else {
		var cfg pyocdConfig
		if err := json5.Unmarshal(cfgBytes, &cfg); err != nil {
			sklog.Errorf("Incorrectly formatted PyOCD config %s", err)
		} else {
			p.deviceType = cfg.DeviceType
		}
	}
	return p
}

func WithHardcodedMachine(deviceType string) pyocdImpl {
	return pyocdImpl{deviceType: deviceType}
}

// DeviceType returns the model identifier of the attached device.
func (p pyocdImpl) DeviceType(ctx context.Context) (string, error) {
	if p.deviceType == "" {
		return "", skerr.Fmt("Empty device type configured")
	}
	output, err := common.TrimmedCommandOutput(ctx, "python3", "-m", "pyocd", "list", "--color=never")
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to get pyocd device type. Output was '%s'", output)
	}
	output = strings.TrimSpace(output)
	if len(output) == 0 {
		return "", skerr.Fmt("Empty output on call to pyocd list")
	}
	// Count how many lines there are. We should expect at least 3 if a device is attached:
	//  header
	// ---------
	// device 0
	lines := strings.Split(output, "\n")
	if len(lines) >= 3 {
		return p.deviceType, nil
	}

	return "", skerr.Fmt("No device attached but %q was expected to be:\n%s", p.deviceType, output)
}

var _ PyOCD = pyocdImpl{}
