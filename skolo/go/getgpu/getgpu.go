package main

// This Windows-only program detects and prints out the GPU vendor / device name and version.
//
// Based on the following Swarming files:
//
// - https://github.com/luci/luci-py/blob/887c873be30051f382da8b6aa8076a7467c80388/appengine/swarming/swarming_bot/api/platforms/win.py#L405
// - https://github.com/luci/luci-py/blob/887c873be30051f382da8b6aa8076a7467c80388/appengine/swarming/swarming_bot/api/platforms/gpu.py#L114

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type VendorID string
type VendorName string
type DeviceID string
type DeviceName string
type VendorDevices struct {
	VendorName VendorName
	Devices    map[DeviceID]DeviceName
}
type PNPDeviceID string // Looks like this: CI\VEN_15AD&DEV_0405&SUBSYS_040515AD&REV_00\3&2B8E0B4B&0&78
type VideoProcessor string
type DriverVersion string

const (
	AMD     = VendorID("1002")
	ASPEED  = VendorID("1a03")
	INTEL   = VendorID("8086")
	MAXTROX = VendorID("102b")
	NVIDIA  = VendorID("10de")
)

var vendorMapping = map[VendorID]VendorDevices{
	AMD: {
		VendorName: "AMD",
		// http://developer.amd.com/resources/ati-catalyst-pc-vendor-id-1002-li
		Devices: map[DeviceID]DeviceName{
			"6613": "Radeon R7 240", // The table is incorrect
			"6646": "Radeon R9 M280X",
			"6779": "Radeon HD 6450/7450/8450",
			"679e": "Radeon HD 7800",
			"67ef": "Radeon RX 560",
			"6821": "Radeon R8 M370X", // 'HD 8800M' or 'R7 M380' based on rev_id
			"683d": "Radeon HD 7700",
			"9830": "Radeon HD 8400",
			"9874": "Carrizo",
		},
	},
	ASPEED: {
		VendorName: "ASPEED",
		Devices: map[DeviceID]DeviceName{
			// https://pci-ids.ucw.cz/read/PC/1a03/2000
			// It seems all ASPEED graphics cards use the same device id
			// (for driver reasons?)
			"2000": "ASPEED Graphics Family",
		},
	},
	INTEL: {
		VendorName: "Intel",
		Devices: map[DeviceID]DeviceName{
			// http://downloadmirror.intel.com/23188/eng/config.xml
			"0046": "Ironlake HD Graphics",
			"0102": "Sandy Bridge HD Graphics 2000",
			"0116": "Sandy Bridge HD Graphics 3000",
			"0166": "Ivy Bridge HD Graphics 4000",
			"0412": "Haswell HD Graphics 4600",
			"041a": "Haswell HD Graphics",
			"0a16": "Intel Haswell HD Graphics 4400",
			"0a26": "Haswell HD Graphics 5000",
			"0a2e": "Haswell Iris Graphics 5100",
			"0d26": "Haswell Iris Pro Graphics 5200",
			"0f31": "Bay Trail HD Graphics",
			"1616": "Broadwell HD Graphics 5500",
			"161e": "Broadwell HD Graphics 5300",
			"1626": "Broadwell HD Graphics 6000",
			"162b": "Broadwell Iris Graphics 6100",
			"1912": "Skylake HD Graphics 530",
			"191e": "Skylake HD Graphics 515",
			"1926": "Skylake Iris 540/550",
			"193b": "Skylake Iris Pro 580",
			"22b1": "Braswell HD Graphics",
			"3e92": "Coffee Lake UHD Graphics 630",
			"5912": "Kaby Lake HD Graphics 630",
			"591e": "Kaby Lake HD Graphics 615",
			"5926": "Kaby Lake Iris Plus Graphics 640",
		},
	},
	MAXTROX: {
		VendorName: "Matrox",
		Devices: map[DeviceID]DeviceName{
			"0522": "MGA G200e",
			"0532": "MGA G200eW",
			"0534": "G200eR2",
		},
	},
	NVIDIA: {
		VendorName: "Nvidia",
		Devices: map[DeviceID]DeviceName{
			// ftp://download.nvidia.com/XFree86/Linux-x86_64/352.21/README/README.txt
			"06fa": "Quadro NVS 450",
			"08a4": "GeForce 320M",
			"08aa": "GeForce 320M",
			"0a65": "GeForce 210",
			"0df8": "Quadro 600",
			"0fd5": "GeForce GT 650M",
			"0fe9": "GeForce GT 750M Mac Edition",
			"0ffa": "Quadro K600",
			"104a": "GeForce GT 610",
			"11c0": "GeForce GTX 660",
			"1244": "GeForce GTX 550 Ti",
			"1401": "GeForce GTX 960",
			"1ba1": "GeForce GTX 1070",
			"1cb3": "Quadro P400",
			"2184": "GeForce GTX 1660",
		},
	},
}

func idsToNames(venID VendorID, defaultVenName VendorName, devID DeviceID, defaultDevName DeviceName) (VendorName, DeviceName) {
	venID = VendorID(strings.ToLower(string(venID)))
	devID = DeviceID(strings.ToLower(string(devID)))

	vendorDevices, ok := vendorMapping[venID]
	if !ok {
		return defaultVenName, defaultDevName
	}

	devName, ok := vendorDevices.Devices[devID]
	if !ok {
		return vendorDevices.VendorName, defaultDevName
	}

	return vendorDevices.VendorName, devName
}

func getGPUInfo() (PNPDeviceID, VideoProcessor, DriverVersion, error) {
	queryWin32VideoController := func(fieldName string) (string, error) {
		ps, err := exec.LookPath("powershell.exe")
		if err != nil {
			return "", err
		}

		// https://msdn.microsoft.com/library/aa394512.aspx
		psCmd := fmt.Sprintf("(Get-CimInstance -ClassName Win32_VideoController | Select-Object %s).%s", fieldName, fieldName)
		cmd := exec.Command(ps, psCmd)

		var stdout bytes.Buffer
		cmd.Stdout = &stdout

		err = cmd.Run()
		if err != nil {
			return "", err
		}

		return strings.TrimSpace(stdout.String()), nil
	}

	pnpDeviceID, err := queryWin32VideoController("PNPDeviceID")
	if err != nil {
		return "", "", "", err
	}
	videoProcessor, err := queryWin32VideoController("VideoProcessor")
	if err != nil {
		return "", "", "", err
	}
	driverVersion, err := queryWin32VideoController("DriverVersion")
	if err != nil {
		return "", "", "", err
	}

	return PNPDeviceID(pnpDeviceID), VideoProcessor(videoProcessor), DriverVersion(driverVersion), nil
}

var (
	vendorIDRegexp = regexp.MustCompile(`VEN_([0-9A-F]{4})`)
	deviceIDRegexp = regexp.MustCompile(`DEV_([0-9A-F]{4})`)
)

func main() {
	pnpDeviceID, videoProcessor, driverVersion, err := getGPUInfo()
	if err != nil {
		panic(err)
	}

	var (
		venID VendorID
		devID DeviceID
	)

	venID = VendorID(vendorIDRegexp.FindStringSubmatch(string(pnpDeviceID))[1])
	if venID == "" {
		venID = "UNKNOWN"
	}

	devID = DeviceID(deviceIDRegexp.FindStringSubmatch(string(pnpDeviceID))[1])
	if devID == "" {
		devID = "UNKNOWN"
	}

	venName, devName := idsToNames(venID, VendorName("Unknown"), devID, DeviceName(videoProcessor))

	if driverVersion != "" {
		fmt.Printf("%s %s %s", venName, devName, driverVersion)
	} else {
		fmt.Printf("%s %s", venName, devName)
	}
}
