#!/bin/bash
set -e

adduser --gecos chrome-bot chrome-bot
adduser pi sudo
install --mode=600 -D --owner=chrome-bot /opt/skolo/authorized_keys /home/chrome-bot/.ssh/authorized_keys
