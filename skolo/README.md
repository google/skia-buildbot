This folder contains code and other resources that help make the Skolo run
smoothly.

The raspberry-pi folder contains Ansible playbooks that can be used to manage
the raspberry pis.

The linux folder contains Ansible playbooks that can be used to manage the
Linux bots in the Skolo.

The /bash/skolo.sh file is a set of shortcuts for interacting with the skolo.
It can be 'source'd from your .bashrc.

    source $GOPATH/src/go.skia.org/infra/skolo/bash/skolo.sh
