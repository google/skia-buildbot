package main

import (
	"flag"
	"fmt"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/censustaker/bots"
	census_common "go.skia.org/infra/skolo/go/censustaker/common"
	"go.skia.org/infra/skolo/go/censustaker/interfaces"
)

var (
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	serviceAccountPath = flag.String("service_account_path", "", "Path to the service account.  Can be empty string to use defaults or project metadata")
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
	// Calls flag.Parse()
	common.InitWithMust(
		"censustaker",
		common.PrometheusOpt(promPort),
		common.CloudLoggingJWTOpt(serviceAccountPath),
	)

	cycle := func() {
		ansible := bots.NewAnsibleBotNameGetter("/usr/local/share/censustaker/")
		edgeswitch := interfaces.NewEdgeSwitchBotPortGetter("192.168.1.41:22")
		bots, err := enumerateBots(ansible, edgeswitch)
		sklog.Infof("Found bots %v (With err %v)", bots, err)
	}

	cycle()
	for range time.Tick(10 * time.Minute) {
		cycle()
	}
}
