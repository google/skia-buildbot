# Switchboard RPi Setup

This directory contains the scripts for configuring RPi SD cards for RPis
running as test machines under Switchboard. See the
[Design Doc](http://go/skia-switchboard).

Debian supports RPi now: https://wiki.debian.org/RaspberryPi. Visit
https://raspi.debian.net/tested-images/ for images.

The image currently used is:

    https://raspi.debian.net/verified/20210629_raspi_4_bullseye.img.xz

This image is backed up at:

    gs://skia-skolo/skolo-images/switchboard/

Download that and burn it to an SD card.

[Balena Etcher](https://www.balena.io/etcher/) is a nice GUI application that
runs on all platforms that allows you to burn multiple SD cards at the same
time.

After it has been burned, reload the SD card and run:

    ./configure-image.sh <machine-name>

Once the SD card has been placed in an RPi and is running in the lab:

1. Add the hostname to //skolo/ansible/hosts.ini, making sure it ends up as part
   of `switchboard_rpis`.
2. Then run the ansible scripts to configure the running RPi:

```bash
     $ cd //skolo/ansible/
     $ ansible-playbook ./switchboard/prepare-rpi-for-ansible.yml \
         --extra-vars variable_hosts=<machine-name>
     $ ansible-playbook ./switchboard/rpi.yml \
         --extra-vars variable_hosts=<machine-name>
```

Now the RPi should be fully setup with adb, idevice-\*, a recent copy of
authorized_keys, and running test_machine_monitor.
