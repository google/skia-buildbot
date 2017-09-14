package main

import (
	"flag"
	"fmt"
	"os"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/censustaker/bots"
	census_common "go.skia.org/infra/skolo/go/censustaker/common"
	"go.skia.org/infra/skolo/go/censustaker/interfaces"
)

var (
	scriptDir     = flag.String("script_dir", "/usr/local/share/censustaker/", "Path in which the ansible scripts and configurations are stored. This will be the working dir when executing the ansible scripts to enumerate all bots on the network.")
	ansibleOutput = flag.String("ansible_out", "/tmp/census_output", "File in which the ansible script should dump its intermediary output.")

	switchAddress = flag.String("switch_address", "", "The IP address of the switch to pull the port numbers from.")
)

func enumerateBots(names bots.BotNameGetter, ports interfaces.BotPortGetter) ([]census_common.Bot, error) {
	nameList, err := names.GetBotNamesAddresses()
	if err != nil {
		return nil, fmt.Errorf("Could not fetch bot names and mac addresses: %s", err)
	}

	portList, err := ports.GetBotPortsAddresses()
	if err != nil {
		return nil, fmt.Errorf("Could not fetch bot ports and mac addresses: %s", err)
	}

	SENTINAL_PORT := -1
	botMap := map[string]census_common.Bot{}
	for _, b := range nameList {
		if b.MACAddress != "" {
			b.Port = SENTINAL_PORT
			botMap[b.MACAddress] = b
		}
	}
	for _, b := range portList {
		if _, ok := botMap[b.MACAddress]; ok && b.MACAddress != "" {
			a := botMap[b.MACAddress]
			a.Port = b.Port
			botMap[b.MACAddress] = a
		}
	}

	botList := []census_common.Bot{}
	for _, b := range botMap {
		if b.Port != SENTINAL_PORT {
			botList = append(botList, b)
		}
	}

	return botList, nil
}

func main() {
	defer common.LogPanic()
	flag.Parse()

	if *scriptDir == "" || *switchAddress == "" {
		sklog.Fatal("--script_dir and --switch_address cannot be empty")
	}

	if _, err := os.Stat(*scriptDir); os.IsNotExist(err) {
		sklog.Fatalf("--script_dir %s points to a non-existent directory", *scriptDir)
	}

	ansible := bots.NewAnsibleBotNameGetter(*scriptDir, *ansibleOutput)
	// TODO(kjlubick): Pull this ip address and the ip address of other switches
	// on the network from the powercycle config file.
	edgeswitch := interfaces.NewEdgeSwitchBotPortGetter(*switchAddress)
	bots, err := enumerateBots(ansible, edgeswitch)
	sklog.Infof("Found bots %v (With err %v)", bots, err)
}
