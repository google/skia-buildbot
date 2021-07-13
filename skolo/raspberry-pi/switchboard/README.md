# Switchboard RPi Setup

This directory contains the scripts for finalizing RPi SD card for production
use in RPi used by Switchboard. See the
[Design Doc](http://go/skia-switchboard).

In this configuration the RPIs are not run under kubernetes, instead are managed
via Ansible.

Debian supports RPi now: https://wiki.debian.org/RaspberryPi Visit
https://raspi.debian.net/tested-images/ for images.

The image currently used is:

    https://raspi.debian.net/verified/20210629_raspi_4_bullseye.img.xz

Download that and burn to an SD card. After it has been burned, reload the SD
card and run:

    ./configure-image.sh <machine-name>

Once the SD cards has been placed in an RPi and is running on the rack, make
sure to add the hostname to //skolo/ansible/hosts.ini and run:

    cd //skolo/ansible/
    ansible-playbook ./switchboard/setup-switchboard-rpi.yml --extra-vars variable_hosts=<machine-name>
    ansible-playbook ./switchboard/install-test-machine-monitor.yml --extra-vars variable_hosts=<machine-name>

Now that device should be fully setup with adb, idevice, and a recent copy of
authorized_keys, and running test_machine_monitor.
