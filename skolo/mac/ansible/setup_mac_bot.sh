#!/bin/bash


if [ "$#" != "1" ]; then
  >&2 echo "Usage: $0 <ip addr>"
  exit 1
fi

ansible-playbook -i "$1," setup-skolo-bot.yml --ask-become-pass --extra-vars="swarming_server=https://chromium-swarm.appspot.com home=/Users/chrome-bot"
