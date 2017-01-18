To set up a new Linux bot, perform the following steps:

Install Ubuntu 16.04, setting up a user `chrome-bot` and a hostname like `skia-e-linux-NNN`.
It is preferable to connect to wifi to allow for updates.

```
sudo apt-get install ansible git
git clone https://github.com/google/skia-buildbot
cd skia-buildbot/skolo/linux/

ifconfig #make note of the ethernet name

sudo ansible-playbook -i "localhost," -c local setup_linux_bot.yml
```

Reboot the machine and connect it to the lab network.

Then, ssh in and bootstrap it to run swarming.