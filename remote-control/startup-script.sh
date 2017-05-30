#! /bin/bash

# Create tmp dir if needed.
sudo -u default mkdir -p /mnt/pd0/tmp

sudo apt-get update
sudo apt-get --assume-yes upgrade
sudo apt-get --assume-yes install novnc
