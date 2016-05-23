#! /bin/bash
set -x

/tmp/format_and_mount.sh imageinfo

set PACKAGES=git build-essential libosmesa-dev libfreetype6-dev libfontconfig-dev libpng12-dev libgif-dev libqt4-dev mesa-common-dev
sudo apt-get update
sudo apt-get --assume-yes upgrade
sudo apt-get --assume-yes install $PACKAGES

# Install depot_tools
cd /mnt/pd0
mkdir imageinfo
git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git
echo -e "\nexport PATH=/mnt/pd0/depot_tools:\$PATH" >> ~/.bashrc

