#!/bin/bash
set -x

adduser --gecos chrome-bot chrome-bot --disabled-password
install --directory --mode=755 --owner=chrome-bot --group=chrome-bot /home/chrome-bot/.ssh
install --mode=600 -D --owner=chrome-bot --group=chrome-bot /opt/skolo/authorized_keys /home/chrome-bot/.ssh/authorized_keys

# Only copy this file over once.
rm /opt/skolo/authorized_keys