package main

import (
	"context"
	"regexp"
	"strings"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/skerr"
)

type arpNameGetter struct{}

func newArpNameGetter() arpNameGetter {
	return arpNameGetter{}
}

// Example line with named groups annotated:
//   skia-rpi-072             ether   b8:27:eb:bc:62:2f   C                     eno1
//   <hostname  >                     <mac_address    >
var arpLineMatcher = regexp.MustCompile(`^(?P<hostname>\S+)\s+ether\s+(?P<mac_address>\S+)\s+`)

// GetDeviceNamesAddresses implements the nameAddressGetter interface.
func (a arpNameGetter) GetDeviceNamesAddresses(ctx context.Context) ([]poeDevice, error) {
	cmd := executil.CommandContext(ctx, "arp")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var devices []poeDevice
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if matches := arpLineMatcher.FindStringSubmatch(line); matches != nil {
			devices = append(devices, poeDevice{
				Hostname:   matches[1],
				MACAddress: strings.ToUpper(matches[2]),
			})
		}
	}
	return devices, nil
}

var _ nameAddressGetter = arpNameGetter{}
