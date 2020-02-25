To set up a new Linux bot, perform the following steps:

Install Ubuntu 16.04, setting up a user `chrome-bot` and a hostname like `skia-e-linux-NNN`.
It is preferable to connect to wifi to allow for updates.

From the machine:

```
ip address #make note of the ethernet name
```

From the jumphost:

```
sudo ansible-playbook -i "${IP_ADDR}," -c local setup_linux_bot.yml
```
