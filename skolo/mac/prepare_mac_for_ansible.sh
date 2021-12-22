#!/bin/bash

if [ "$#" != "1" ]; then
  >&2 echo "Usage: $0 <hostname>"
  exit 1
fi

ansible-playbook -i ../ansible/hosts.yml -l "$1," ../ansible/switchboard/prepare-mac-for-ansible.yml --ask-pass --ask-become-pass --extra-vars="mac_hostname=$1"
