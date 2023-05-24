# Environment Monitor

Environment Monitor uses an attached sensor module to report various environmental
readings to be recorded in metrics2.

The reported metrics are:

1. Ambient temperature (C).
2. Humidity (%).
3. Ambient Light (Unit value 0..1).
4. Sound level (dB).

This monitor currently uses the [DLP-TH1C](https://www.dlpdesign.com/usb/th1c.php)
sensor module which is connected to the host via USB. The //skolo/go/sensors package
communicates with the DLP-TH1C over serial.

## Local Development

To run this locally a DLP-TH1C sensor module needs to be connected to your
workstation. It can be run from the `//skolo` directory as so:

```command
$ go run go/environment_monitor_ansible/main.go \
  -serial_device=/path/to/serial/device -metric_prefix=<prefix>
```

for example:

Here `/dev/ttyACM0` is the sensor module device - yours will likely differ.

```command
$ go run go/environment_monitor_ansible/main.go \
  -serial_device=/dev/ttyACM0 -metric_prefix=testing_
```

The metrics, which take ~30 seconds to get their first reading, can be read
by:

```command
$ curl -s http://localhost:20000/metrics
```

There is also a sensor library tool in `//skolo/go/sensors_tool`.

## Deployment

For instructions on deploying this, see
//skolo/ansible/switchboard/roles/install_environment_monitor/README.md
