#!/bin/bash


if [ "$#" != "1" ]; then
  >&2 echo "Usage: $0 <ip addr>"
  exit 1
fi

ansible-playbook -i mac_hosts setup-skolo-bot.yml --extra-vars "ip_addr=${1}"
