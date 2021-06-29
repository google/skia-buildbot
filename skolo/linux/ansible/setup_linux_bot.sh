#!/bin/bash


if [ "$#" != "1" ]; then
  >&2 echo "Usage: $0 <ip addr>"
  exit 1
fi

ansible-playbook -i "$1," setup-skolo-bot.yml --ask-pass --ask-become-pass --extra-vars="swarming_server=https://chromium-swarm.appspot.com home=/home/chrome-bot"
