package main

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
)

// The BotNameGetter interface abstracts the logic to collect all information
// about the bots (e.g. Name, IPv4 Address) except the EdgeSwitch ports.
type BotNameGetter interface {
	// Collects and returns a list of Bot with nearly all the
	// information filled out. This can be a very expensive call
	// (2 minutes or more).
	GetBotNamesAddresses() ([]Bot, error)
}

// Implements BotNameGetter using Ansible.
type ansibleBotSource struct {
	scriptDir  string
	outputFile string
}

// NewAnsibleBotNameGetter returns an Ansible implemented version of BotNameGetter
func NewAnsibleBotNameGetter(scriptDir, outputFile string) *ansibleBotSource {
	return &ansibleBotSource{
		scriptDir:  scriptDir,
		outputFile: outputFile,
	}
}

// GetBotNamesAddresses, fulfills BotNameGetter
func (a *ansibleBotSource) GetBotNamesAddresses() ([]Bot, error) {
	if err := os.Remove(a.outputFile); err != nil {
		sklog.Warningf("Could not clear out file: %s", err)
	}
	output := bytes.Buffer{}
	// This command scans all hosts on this network (specified by sys/all-hosts)
	// and records the hostname, mac address and ip address of them all.
	err := exec.Run(&exec.Command{
		Name: "ansible-playbook",
		Args: []string{"-i", "all-hosts", "enumerate_hostnames.yml",
			"--extra-vars", "output_file=" + a.outputFile, "-vv"},
		Dir:            a.scriptDir,
		CombinedOutput: &output,
		// The task usually takes about 2 minutes. 10 is very generous in case of network
		// or load congestion.
		Timeout: 10 * time.Minute,
	})
	sklog.Warningf("Possible error while running ansible command: %v", err)
	sklog.Infof("Output from ansible command: %s", output.String())

	content, err := ioutil.ReadFile(a.outputFile)
	if err != nil {
		return nil, fmt.Errorf("Could not get output from ansible: %s", err)
	}
	return parseAnsibleResult(string(content)), nil
}

var ansibleLine = regexp.MustCompile(`^(?P<hostname>\S+)\s+(?P<ipv4_address>[0-9\.]+)\s+(?P<mac_address>[0-9A-Fa-f\:]+)`)

// parseAnsibleResult looks at the output from the enumerate_hostnames.yml call
// which is simply a space separated tuple of hostname, ip address, and mac address
func parseAnsibleResult(input string) []Bot {
	lines := strings.Split(input, "\n")
	bots := []Bot{}
	for _, l := range lines {
		if matches := ansibleLine.FindStringSubmatch(l); matches != nil {
			bots = append(bots, Bot{Hostname: matches[1], IPV4Address: matches[2], MACAddress: strings.ToUpper(matches[3])})
		}
	}
	return bots
}
