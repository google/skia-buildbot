package interfaces

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.skia.org/infra/skolo/go/censustaker/common"
	"go.skia.org/infra/skolo/go/powercycle"
)

type BotPortGetter interface {
	GetBotPortsAddresses() ([]common.Bot, error)
}

type edgeswitchBotSource struct {
	client powercycle.EdgeSwitchCommandRunner
}

//b2, err := c.ExecCmds([]string{"show mac-addr-table all"})

func NewEdgeSwitchBotPortGetter(address string) *edgeswitchBotSource {
	return &edgeswitchBotSource{client: powercycle.NewEdgeSwitchClient(address)}
}

func (e *edgeswitchBotSource) GetBotPortsAddresses() ([]common.Bot, error) {
	lines, err := e.client.ExecCmds([]string{"show mac-addr-table all"})
	if err != nil {
		return nil, err
	}
	bots, err := parseSSHResult(lines)
	if err != nil {
		return nil, err
	}
	return dedupeBots(bots), nil
}

var edgeswitchLine = regexp.MustCompile(`^\S+\s+(?P<mac_address>[0-9A-Fa-f\:]+)\s+\S+\s+(?P<interface>\d+)\s+\S+`)

func parseSSHResult(lines []string) ([]common.Bot, error) {
	bots := []common.Bot{}
	for _, l := range lines {
		if matches := edgeswitchLine.FindStringSubmatch(l); matches != nil {
			port, err := strconv.ParseInt(matches[2], 10, 0)
			if err != nil {
				return nil, fmt.Errorf("Unexpected formatting error. %s is not an int: %s", matches[2], err)
			}
			bots = append(bots, common.Bot{MACAddress: strings.ToUpper(matches[1]), Port: int(port)})
		}
	}
	return bots, nil
}

// dedupeBots filters out the list of bots such that only bots whose port
// assignments are unique are in the list. Bots with duplicate ports are
// likely not directly attached to this switch.
func dedupeBots(bots []common.Bot) []common.Bot {
	uniquePorts := map[int]bool{}
	for _, b := range bots {
		if _, ok := uniquePorts[b.Port]; ok {
			uniquePorts[b.Port] = false
		} else {
			uniquePorts[b.Port] = true
		}
	}

	unique := []common.Bot{}
	for _, b := range bots {
		if uniquePorts[b.Port] {
			unique = append(unique, b)
		}
	}
	return unique
}
