# RPi Management With Kubernetes

To scale our deployment of Android and iPhone test devices we need an easier and
faster way to deploy, manage, and monitor RPis and their attached devices.

This design presumes that the jupmhost management outlined in
[DESIGN](./DESIGN.md) is complete. The RPis will be added to the k3s cluster
managed by the jumphost.

## rpi-swarming-client

This image is the image that runs swarming on the RPi. It contains most of the
same machinery that the current RPi bots contain.

- `/start_swarming` decides to start swarming_bot.zip or download it via bootstrap.py.
- `/opt/swarming/bootstrap.py` bootstraps the copy of swarming_bot.zip
- `/etc/swarming_config/oauth2_access_token_config.json` tells swarming where
  to get auth tokens.
- `/usr/bin/bot_config` is our Go application that replaces the functionality of
  bot_config.py.

The image runs as root and in privileged mode so that it can run adb.

The rpi-swarming-client image is run as a daemonset so that each and every RPi
runs just one copy of the image. See the YAML file for the selectors that also
stop it from running on the jumphost.
