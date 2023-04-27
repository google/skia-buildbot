// Environment Monitor reports ambient environment sensor values
// to be recorded in metrics2.
package main

import (
	"flag"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/sensors"
)

var (
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	serialDevice = flag.String("serial_device", "", "Serial device (e.g., '/dev/ttyACM0' or 'COM1')")
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

func main() {
	common.InitWithMust(
		"environment-monitor",
		common.PrometheusOpt(promPort),
	)

	if *serialDevice == "" {
		sklog.Fatal(`"serial_device" is a required parameter.`)
	}

	d, err := openDevice(*serialDevice)
	if err != nil {
		sklog.Fatal(err)
	}
	tempMetric := metrics2.GetFloat64Metric("temp_c")
	humidityMetric := metrics2.GetFloat64Metric("humidity")
	lightMetric := metrics2.GetFloat64Metric("light")
	soundMetric := metrics2.GetFloat64Metric("sound_db")
	for range time.Tick(time.Minute) {
		t, err := d.GetTemperature()
		if err != nil {
			tempMetric.Update(float64(t))
		} else {
			sklog.Fatal("Error reading temperature: %s", err)
		}
		h, err := d.GetHumidity()
		if err != nil {
			humidityMetric.Update(float64(h))
		} else {
			sklog.Fatal("Error reading humidity: %s", err)
		}
		light, err := d.GetLight()
		if err != nil {
			lightMetric.Update(float64(light))
		} else {
			sklog.Fatal("Error reading light level: %s", err)
		}
		s, err := d.GetBroadbandSound()
		if err != nil {
			soundMetric.Update(float64(s))
		} else {
			sklog.Fatal("Error reading sound level: %s", err)
		}
	}
}
