// The censustaker executable combines data from multiple sources to generate a list of devices
// which are attached to a given Ubiquiti EdgeSwitch. The switch can only tell us which mac
// addresses are attached to which ports, so we need another source of data to give us a list of
// hostnames and ip addresses to be able to generate the mapping of hostname to port number needed
// by powercycle.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skolo/go/powercycle"
)

type poeDevice struct {
	Hostname   string
	POEPort    int
	MACAddress string
}

// nameAddressGetter abstracts the logic to collect all information about the machines except
// for which EdgeSwitch ports they are attached to.
type nameAddressGetter interface {
	// GetDeviceNamesAddresses returns a []poeDevice with the names and mac addresses filled out.
	GetDeviceNamesAddresses(context.Context) ([]poeDevice, error)
}

// addressPortGetter abstracts the logic to collect the EdgeSwitch ports to which our devices
// are connected.
type addressPortGetter interface {
	// GetDevicePortsAddresses returns a []poeDevice with the mac addresses and ports filled out.
	GetDevicePortsAddresses(context.Context) ([]poeDevice, error)
}

func combineSources(ctx context.Context, names nameAddressGetter, ports addressPortGetter) ([]poeDevice, error) {
	nameList, err := names.GetDeviceNamesAddresses(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching device names and mac addresses")
	}

	portList, err := ports.GetDevicePortsAddresses(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching device ports and mac addresses")
	}

	const sentinelPort = -1
	byMacAddress := map[string]poeDevice{}
	for _, b := range nameList {
		if b.MACAddress != "" {
			b.POEPort = sentinelPort
			byMacAddress[b.MACAddress] = b
		}
	}
	for _, b := range portList {
		if _, ok := byMacAddress[b.MACAddress]; ok && b.MACAddress != "" {
			a := byMacAddress[b.MACAddress]
			a.POEPort = b.POEPort
			byMacAddress[b.MACAddress] = a
		}
	}

	var devices []poeDevice
	for _, b := range byMacAddress {
		if b.POEPort != sentinelPort {
			devices = append(devices, b)
		}
	}

	return devices, nil
}

func makeConfig(ctx context.Context, address, user, password string, hostnameMatcher *regexp.Regexp) (powercycle.EdgeSwitchConfig, error) {
	output := powercycle.EdgeSwitchConfig{
		Address:    address,
		User:       user,
		Password:   "", // leave this blank so as not to leak it
		DevPortMap: map[powercycle.DeviceID]int{},
	}
	arp := newArpNameGetter()
	edgeswitch := newSwitchPortGetter(address, user, password)
	devices, err := combineSources(ctx, arp, edgeswitch)
	if err != nil {
		return output, skerr.Wrap(err)
	}
	for _, device := range devices {
		if hostnameMatcher.MatchString(device.Hostname) {
			output.DevPortMap[powercycle.DeviceID(device.Hostname)] = device.POEPort
		}
	}
	return output, nil
}

func main() {
	var (
		switchAddress  = flag.String("switch_address", "", "The IP address of the switch to pull the port numbers from.")
		switchUser     = flag.String("switch_user", "power", "Username of the switch")
		switchPassword = flag.String("switch_password", "", "password for the switch user")
		hostnameRegex  = flag.String("hostname_regex", "rpi", "Regex to match hostnames for")
	)
	flag.Parse()

	ctx := context.Background()
	out, err := makeConfig(ctx, *switchAddress, *switchUser, *switchPassword, regexp.MustCompile(*hostnameRegex))
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Printf("Error making JSON: %s\n", err)
		os.Exit(2)
	}
	fmt.Println(string(b))
}
