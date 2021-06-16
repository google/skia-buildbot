# RPi Management With Kubernetes

To scale our deployment of Android and iPhone test devices we need an easier and
faster way to deploy, manage, and monitor RPis and their attached devices.

This design presumes that the jupmhost management outlined in
[DESIGN](./DESIGN.md) is complete. The RPis will be added to the k3s cluster
managed by the jumphost.

## rpi-swarming-client

This image is the image that runs swarming on the RPi. It contains most of the
same machinery that the current RPi bots contain.

- `/etc/swarming_config/oauth2_access_token_config.json` tells swarming where to
  get auth tokens.
- `/usr/bin/test_machine_monitor` is our Go application that handles downloading
  the `swarming_bot.zip` file from swarming and running it as a sub-process.

For RPis that are running under this system their names begin with "skia-rpi2",
which causes them to use the `skia_mobile2.py` script. This is a very simple
script that just bounces all swarming test_machine_monitor requests back to the
Golang test_machine_monitor as HTTP requests. That allows test_machine_monitor
to be a long running process that can emit logs and also report metrics.

    test_machine_monitor        Swarming Server         python swarming_bot.zip
        +                     +                          +
        |                     |                          |
        |                     |                          |
        +-------------------->+                          |
        |   Download          |                          |
        |    swarming_bot.zip |                          |
        |                     |                          |
        |                     |                          |
        +----------------------------------------------->+
        |                     |       Launch()           |
        |                     |                          |
        |                     |                          |
        |                     |                          |
        +<-----------------------------------------------+
        |                     |    HTTP Requests         |
        |                     |      /get_state          |
        |                     |      /get_dimensions     |
        |                     |      /get_settings       |
        |                     |                          |
        |                     |                          |

The image runs as root and in privileged mode so that it can run adb.

The rpi-swarming-client image is run as a daemonset so that each and every RPi
runs just one copy of the image. See the YAML file for the selectors that also
stop it from running on the jumphost.

There are a couple files in the image that should be highlighted:

**sudo**

The swarming client is very opinionated and believes it should be able to reboot
the machine any time it wants. This is the only time the swarming client
attempts to use or needs to use sudo. A typical swarming install on a normal
machine adds a `sudo reboot` to the sudoers file to accomplish this. Since we
are running in a container this is a useless feature and we take that away
swarming by adding a `sudo` script on the PATH, which is just a simple bash
script that `kill -9`s any process that runs the script. Since the image runs as
root a user that `kubectl exec`s into a running image doesn't need to use sudo.

**qemu-arm-static**

This file allows an arm7 image to be run on x86 machine, allowing the image
build process to be run on an x86 desktop and the resulting image to also be run
on an x86 desktop as well as an arm7 device. Note that anything to do with
running adb will probably not work on an x86 machine.
