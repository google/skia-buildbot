# Switchboard RPi Setup

This directory contains the scripts for configuring RPi SD cards for RPis
running as test machines under Switchboard. See the
[Design Doc](http://go/skia-switchboard).

Debian supports RPi now: https://wiki.debian.org/RaspberryPi. Visit
https://raspi.debian.net/tested-images/ for images.

The image currently used is:

    https://raspi.debian.net/verified/20210629_raspi_4_bullseye.img.xz

Download that and burn it to an SD card. After it has been burned, reload the SD
card and run:

    ./configure-image.sh <machine-name>

Once the SD card has been placed in an RPi and is running in the lab:

1. Add the hostname to //skolo/ansible/hosts.ini.
2. Run:

```bash
     $ cd //skolo/ansible/
     $ ansible-playbook ./switchboard/setup-switchboard-rpi.yml \
         --extra-vars variable_hosts=<machine-name>

     $ ansible-playbook ./switchboard/install-test-machine-monitor.yml \
         --extra-vars variable_hosts=<machine-name>
```

Now the RPi should be fully setup with adb, idevice-\*, a recent copy of
authorized_keys, and running test_machine_monitor.
