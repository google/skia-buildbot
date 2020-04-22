package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skolo/go/powercycle"
)

type edgeswitchPortGetter struct {
	client powercycle.CommandRunner
}

// newSwitchPortGetter returns an implementation of addressPortGetter
func newSwitchPortGetter(address, user, password string) *edgeswitchPortGetter {
	target := fmt.Sprintf("%s@%s", user, address)
	return &edgeswitchPortGetter{
		client: powercycle.PasswordSSHCommandRunner(password, target),
	}
}

// GetDevicePortsAddresses implements addressPortGetter
func (e *edgeswitchPortGetter) GetDevicePortsAddresses(ctx context.Context) ([]poeDevice, error) {
	// ignore error because when talking to the edgeswitch, sshpass always returns an error
	out, _ := e.client.ExecCmds(ctx,
		"enable",
		"show mac-addr-table all",
		"mmmmmmmmm", // by default, the table only shows the first 20 entries. If we hit m, we see
		// another 20. This is enough to see 200 entries, which should be plenty.
	)
	// It's hard to tell, but the raw output from the edgeswitch is full of carriage returns (\r),
	// which can mess up regex searching. Treating them as newlines for the purpose of processing
	// makes that easier.
	out = strings.Replace(out, "\r", "\n", -1)
	bots, err := parseSSHResult(strings.Split(out, "\n"))
	if err != nil || len(bots) == 0 {
		return nil, skerr.Wrapf(err, "parsing output from (possibly failed command): \n%s\n", out)
	}
	return dedupeBots(bots), nil
}

// Example line with named groups annotated:
//   1        b8:27:eb:83:06:91   0/42                   42       Learned
//            <mac_address    >                      <interface>
var edgeswitchLine = regexp.MustCompile(`^\S+\s+(?P<mac_address>[0-9A-Fa-f:]+)\s+\S+\s+(?P<interface>\d+)\s+\S+`)

// parseSSHResult looks at the lines output by the EdgeSwitchClient. These are
// already split by \n.  It then parses the lines into the various components.
// See the unit tests for an example of what this data looks like.
func parseSSHResult(lines []string) ([]poeDevice, error) {
	var devices []poeDevice
	for _, l := range lines {
		if matches := edgeswitchLine.FindStringSubmatch(l); matches != nil {
			port, err := strconv.ParseInt(matches[2], 10, 0)
			if err != nil {
				return nil, skerr.Wrapf(err, "formatting error. %s is not an int", matches[2])
			}
			devices = append(devices, poeDevice{
				MACAddress: strings.ToUpper(matches[1]),
				POEPort:    int(port),
			})
		}
	}
	return devices, nil
}

// dedupeBots filters out the list of bots such that only bots whose port
// assignments are unique are in the list. Bots with duplicate ports are
// likely not directly attached to this switch.
func dedupeBots(bots []poeDevice) []poeDevice {
	uniquePorts := map[int]bool{}
	for _, b := range bots {
		if _, ok := uniquePorts[b.POEPort]; ok {
			uniquePorts[b.POEPort] = false
		} else {
			uniquePorts[b.POEPort] = true
		}
	}

	var unique []poeDevice
	for _, b := range bots {
		if uniquePorts[b.POEPort] {
			unique = append(unique, b)
		}
	}
	return unique
}

var _ addressPortGetter = (*edgeswitchPortGetter)(nil)
