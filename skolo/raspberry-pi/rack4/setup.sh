#!/bin/bash
set -e

adduser --gecos chrome-bot chrome-bot
adduser pi sudo
echo "skolo/authorized_keys" > /home/chrome-bot/.ssh/authorized_keys
