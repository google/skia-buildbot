// Command-line utility application to read sensor values and printSensorValue to stdout.
//
// Currently only supports the DLP-TH1C sensor module.
package main

import (
	"fmt"
	"os"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skolo/go/sensors"
)

func openDevice(portName string) (*sensors.DLPTH1C, error) {
	d, err := sensors.NewDLPTH1C(portName)
	if err != nil {
		return nil, skerr.Wrapf(err, `error opening port "%s"`, portName)
	}
	const maxPings = 5
	err = d.ConfirmConnection(maxPings)
	if err != nil {
		return nil, skerr.Wrapf(err, "error confirming connection to device")
	}
	return d, nil
}

func printSensorValue[S fmt.Stringer](label string, fn func() (S, error)) {
	s, err := fn()
	if err != nil {
		fmt.Printf("Error %s: %s\n", label, err)
	} else {
		fmt.Printf("%s: %v\n", label, s)
	}
}

func main() {
	args := os.Args[1:]
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s <serial-port-name>\n", os.Args[0])
		os.Exit(1)
	}

	portName := args[0]
	d, err := openDevice(portName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open device: %v\n", err)
		os.Exit(2)
	}

	printSensorValue("Temperature", d.GetTemperature)
	printSensorValue("Humidity", d.GetHumidity)
	printSensorValue("Pressure", d.GetPressure)
	printSensorValue("Tilt", d.GetTilt)
	printSensorValue("Vibration X", d.GetVibrationX)
	printSensorValue("Vibration Y", d.GetVibrationY)
	printSensorValue("Vibration Z", d.GetVibrationZ)
	printSensorValue("Light", d.GetLight)
	printSensorValue("Sound", d.GetSound)
	printSensorValue("Broadband sound", d.GetBroadbandSound)
}
