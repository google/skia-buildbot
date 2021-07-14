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

     $ cd //machine
     $ make build_test_machine_monitor_rpi
     $ TARGET=<machine-name> make push_test_machine_monitor_rpi
```

Now the RPi should be fully setup with adb, idevice-\*, a recent copy of
authorized_keys, and running test_machine_monitor.

## Testing outside the skolo.

_This only applies to testing RPis outside the skolo._

Outside the skolo the RPi will not have a metadata server to talk to, so
`test_machine_monitor` will fail to run. You can get `test_machine_monitor` to
run by supplying another form of Google Application Credentials. One way is to
create a service account key, copy it over the RPI, and then set an Env variable
in the `test_machine_monitor.service` file.

1.  Use an existing service account key, or visit the cloud console page and
    generate a new key.
2.  Copy that key over to the RPi:

        scp key.json chrome-bot@192.168.1.107:/home/chrome-bot/key.json

3.  Update the `test_machine_monitor.service` to add the
    `GOOGLE_APPLICATION_CREDENTIALS` environment variable that points to the
    service account key.

        [Unit]
        Description=test_machine_monitor
        After=syslog.target network.target

        [Service]
        Type=simple
        User=chrome-bot
        Environment=GOOGLE_APPLICATION_CREDENTIALS=/home/chrome-bot/key.json
        ExecStart=/usr/local/bin/test_machine_monitor \
           --start_switchboard
        Restart=always

        [Install]
        WantedBy=multi-user.target

4.  Force the new config to be loaded:

        systemctl daemon-reload

5.  Restart the service

        systemctl restart test_machine_monitor
