#!/bin/bash
# Set up a Mac before it's had its ssh keys installed. This is just like
# updating an existing Mac except that we use passwords and we reboot afterward.
set -e

if [ "$#" != "1" ]; then
  >&2 echo "Usage: $0 <hostname>"
  exit 1
fi

ansible-playbook -i ../ansible/hosts.yml -l "$1," ../ansible/switchboard/mac.yml --ask-pass --ask-become-pass
# Reboot so screensaver disabling takes effect; it's the cheapest way to log out and in again:
ansible -i ../ansible/hosts.yml -m reboot --become "$1,"
