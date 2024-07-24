// Package pyocd exposes routines for communicating with locally attached devices via
// the pyocd CLI tool.
package pyocd

import (
	"context"
	"os"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
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
	// We do not use pyocd to actually check for the device because, the way
	// tests currently run, we shut off the device after every task to keep
	// it from going awry.
	return p.deviceType, nil
}

var _ PyOCD = pyocdImpl{}
