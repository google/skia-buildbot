package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.skia.org/infra/skolo/go/powercycle"
)

// The BotNameGetter interface abstracts the logic to collect the EdgeSwitch ports
// to which our devices are connected.
type BotPortGetter interface {
	// Collects and returns a list of Bot with just the MAC addresses
	// and port numbers. This is a cheap call (<10 seconds).
	GetBotPortsAddresses() ([]Bot, error)
}

// Implements BotPortGetter
type edgeswitchBotSource struct {
	client powercycle.CommandRunner
}

// NewEdgeSwitchBotPortGetter returns an implementation of BotPortGetter
func NewEdgeSwitchBotPortGetter(address string) *edgeswitchBotSource {
	target := fmt.Sprintf("ubnt@%s", address)
	return &edgeswitchBotSource{
		// TODO(kjlubick) pipe the password through from the CLI
		client: powercycle.PasswordSSHCommandRunner("Not-the-real-password", target),
	}
}

// GetBotPortsAddresses, see BotPortGetter
func (e *edgeswitchBotSource) GetBotPortsAddresses() ([]Bot, error) {
	lines, err := e.client.ExecCmds(context.TODO(), "show mac-addr-table all")
	if err != nil {
		return nil, err
	}
	bots, err := parseSSHResult(strings.Split(lines, "\n"))
	if err != nil {
		return nil, err
	}
	return dedupeBots(bots), nil
}

var edgeswitchLine = regexp.MustCompile(`^\S+\s+(?P<mac_address>[0-9A-Fa-f:]+)\s+\S+\s+(?P<interface>\d+)\s+\S+`)

// parseSSHResult looks at the lines output by the EdgeSwitchClient. These are
// already split by \n.  It then parses the lines into the various components.
// See the unit tests for an example of what this data looks like.
func parseSSHResult(lines []string) ([]Bot, error) {
	bots := []Bot{}
	for _, l := range lines {
		if matches := edgeswitchLine.FindStringSubmatch(l); matches != nil {
			port, err := strconv.ParseInt(matches[2], 10, 0)
			if err != nil {
				return nil, fmt.Errorf("Unexpected formatting error. %s is not an int: %s", matches[2], err)
			}
			bots = append(bots, Bot{MACAddress: strings.ToUpper(matches[1]), Port: int(port)})
		}
	}
	return bots, nil
}

// dedupeBots filters out the list of bots such that only bots whose port
// assignments are unique are in the list. Bots with duplicate ports are
// likely not directly attached to this switch.
func dedupeBots(bots []Bot) []Bot {
	uniquePorts := map[int]bool{}
	for _, b := range bots {
		if _, ok := uniquePorts[b.Port]; ok {
			uniquePorts[b.Port] = false
		} else {
			uniquePorts[b.Port] = true
		}
	}

	unique := []Bot{}
	for _, b := range bots {
		if uniquePorts[b.Port] {
			unique = append(unique, b)
		}
	}
	return unique
}
