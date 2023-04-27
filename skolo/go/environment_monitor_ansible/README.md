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

For instructions on deploying this, see
//skolo/ansible/switchboard/roles/install_environment_monitor/README.md
