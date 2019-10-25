package powercycle

import (
	"fmt"
	"sort"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// SeeeduinoConfig contains the necessary parameters to connect
// and control an Arduino with servos via the serial_relay program, that
// is assumed to run on the remote host.
type SeeeduinoConfig struct {
	Address    string         `json:"address"`  // IP address of the device, i.e. 192.168.1.33
	BaseURL    string         `json:"base_url"` // Base url that signals a reboot "/arduino/reboot/"
	DevPortMap map[string]int `json:"ports"`    // Mapping between device name and port
}

// SeeeduinoClient implements the DeviceGroup interface.
type SeeeduinoClient struct {
	deviceIDs       []string
	seeeduinoConfig *SeeeduinoConfig
}

// NewSeeeduinoClient returns a new instance of DeviceGroup for the
// Arduino driven servos.
func NewSeeeduinoClient(config *SeeeduinoConfig, connect bool) (DeviceGroup, error) {
	if connect {
		if _, err := httputils.NewTimeoutClient().Get(config.Address); err != nil {
			return nil, fmt.Errorf("Unable to connect to Seeduino: %v", err)
		}
		sklog.Infof("Connection successful")
	}

	devIDs := make([]string, 0, len(config.DevPortMap))
	for id := range config.DevPortMap {
		devIDs = append(devIDs, id)
	}
	sort.Strings(devIDs)

	ret := &SeeeduinoClient{
		deviceIDs:       devIDs,
		seeeduinoConfig: config,
	}

	return ret, nil
}

// DeviceIDs, see DeviceGroup interface.
func (s *SeeeduinoClient) DeviceIDs() []string {
	return s.deviceIDs
}

// PowerCycle, see PowerCycle interface.
func (s *SeeeduinoClient) PowerCycle(devID string, delayOverride time.Duration) error {
	if !util.In(devID, s.deviceIDs) {
		return fmt.Errorf("Unknown device ID: %s", devID)
	}

	port := s.seeeduinoConfig.DevPortMap[devID]
	url := fmt.Sprintf("%s%s%d", s.seeeduinoConfig.Address, s.seeeduinoConfig.BaseURL, port)
	if resp, err := httputils.NewTimeoutClient().Get(url); err != nil {
		sklog.Infof("Status was %s", resp.Status)
		return fmt.Errorf("Unable to connect to Seeduino to reboot: %v", err)
	} else {
		sklog.Infof(`Response was %s - "500 OK" means everything was good, the HTTP response just responded early.`, resp.Status)
		sklog.Debug(httputils.ReadAndClose(resp.Body))
	}
	// The request returns early, so we make sure to wait so we don't
	// trigger multiple servos at once.
	time.Sleep(10 * time.Second)

	sklog.Infof("Powercycled port %s on port %d.", devID, port)
	return nil
}

// PowerUsage, see the DeviceGroup interface.
func (s *SeeeduinoClient) PowerUsage() (*GroupPowerUsage, error) {
	return &GroupPowerUsage{}, nil // N/A for arduino board.
}
