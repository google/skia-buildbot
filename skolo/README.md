This folder contains code and other resources that help make the Skolo run smoothly.

The raspberry-pi folder contains Ansible playbooks that can be used to manage the raspberry pis and the rpi-master.

The linux folder contains Ansible playbooks that can be used to manage the Linux bots in the Skolo.

The go folder contains the source code for small utility code that runs either on the rpi-master or the raspberry pis themselves.
This code is deployed, like all other infra code, using push/pull, via the master.

The list of utilities are:
  - hotspare: the utility that allows for a hot spare of the master to become live when the master fails.  Build with `make hotspare`