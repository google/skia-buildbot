package bots

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/censustaker/common"
)

type BotNameGetter interface {
	GetBotNamesAddresses() ([]common.Bot, error)
}

// Implements BotNameGetter
type ansibleBotSource struct {
	scriptDir string
}

func NewAnsibleBotNameGetter(scriptDir string) *ansibleBotSource {
	return &ansibleBotSource{scriptDir: scriptDir}
}

func (a *ansibleBotSource) GetBotNamesAddresses() ([]common.Bot, error) {
	if err := os.Remove("/tmp/census_output"); err != nil {
		sklog.Warningf("Could not clear out file: %s", err)
	}
	output := bytes.Buffer{}
	err := exec.Run(&exec.Command{
		Name:           "ansible-playbook",
		Args:           []string{"-i", "all-hosts", "enumerate_hostnames.yml", "-vv"},
		Dir:            a.scriptDir,
		CombinedOutput: &output,
		Timeout:        10 * time.Minute,
	})
	sklog.Warningf("Possible error while running ansible command: %v", err)
	sklog.Infof("Output from ansible command: %s", output.String())

	content, err := ioutil.ReadFile("/tmp/census_output")
	if err != nil {
		return nil, fmt.Errorf("Could not get output from ansible: %s", err)
	}
	return parseAnsibleResult(string(content)), nil
}

var ansibleLine = regexp.MustCompile(`^(?P<hostname>\S+)\s+(?P<ipv4_address>[0-9\.]+)\s+(?P<mac_address>[0-9A-Fa-f\:]+)`)

func parseAnsibleResult(input string) []common.Bot {
	lines := strings.Split(input, "\n")
	bots := []common.Bot{}
	for _, l := range lines {
		if matches := ansibleLine.FindStringSubmatch(l); matches != nil {
			bots = append(bots, common.Bot{Hostname: matches[1], IPV4Address: matches[2], MACAddress: strings.ToUpper(matches[3])})
		}
	}
	return bots
}
