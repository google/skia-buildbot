// Environment Monitor reports ambient environment sensor values
// to be recorded in metrics2.
package main

import (
	"flag"
	"fmt"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/sensors"
)

var (
	promPort      = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	serialDevice  = flag.String("serial_device", "", "Serial device (e.g., '/dev/ttyACM0' or 'COM1')")
	metricsPrefix = flag.String("metric_prefix", "", "String to prefix metric names (e.g., 'skolo_')")
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
	if *metricsPrefix == "" {
		sklog.Fatal(`"metric_prefix" is a required parameter.`)
	}

	d, err := openDevice(*serialDevice)
	if err != nil {
		sklog.Fatal(err)
	}
	tempMetric := metrics2.GetFloat64Metric(fmt.Sprintf("%stemp_c", *metricsPrefix))
	humidityMetric := metrics2.GetFloat64Metric(fmt.Sprintf("%shumidity", *metricsPrefix))
	lightMetric := metrics2.GetFloat64Metric(fmt.Sprintf("%slight", *metricsPrefix))
	soundMetric := metrics2.GetFloat64Metric(fmt.Sprintf("%ssound_db", *metricsPrefix))
	for range time.Tick(time.Minute) {
		t, err := d.GetTemperature()
		if err != nil {
			sklog.Fatalf("Error reading temperature: %s", err)
		}
		tempMetric.Update(float64(t))
		h, err := d.GetHumidity()
		if err != nil {
			sklog.Fatalf("Error reading humidity: %s", err)
		}
		humidityMetric.Update(float64(h))
		light, err := d.GetLight()
		if err != nil {
			sklog.Fatalf("Error reading light level: %s", err)
		}
		lightMetric.Update(float64(light))
		s, err := d.GetBroadbandSound()
		if err != nil {
			sklog.Fatalf("Error reading sound level: %s", err)
		}
		soundMetric.Update(float64(s))
	}
}
