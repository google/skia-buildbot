#! /bin/bash
set -x
sudo apt-get update
sudo apt-get --assume-yes upgrade
sudo apt-get install systemd-container, git

# Install depot_tools
cd /mnt/pd0
mkdir fiddle
git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git
echo -e "\nexport PATH=/mnt/pd0/depot_tools:\$PATH" >> ~/.bashrc

