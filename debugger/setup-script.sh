#! /bin/bash
set -e
set -x

# The same set of packages need to be installed both on the instance and within the container.
PACKAGES="systemd-container git debootstrap build-essential libosmesa-dev libfreetype6-dev libfontconfig-dev libpng12-dev libgif-dev libqt4-dev mesa-common-dev"
sudo apt-get update
sudo apt-get --assume-yes upgrade
sudo apt-get --assume-yes install ${PACKAGES}

# Install depot_tools
cd /mnt/pd0
mkdir debugger
mkdir debugger/out
git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git
echo -e "\nexport PATH=/mnt/pd0/depot_tools:\$PATH" >> ~/.bashrc

# Build the containter
CONTAINER=/mnt/pd0/container
sudo debootstrap --arch=amd64 wily --include=${PACKAGES// /,} /mnt/pd0/container

sudo rm $CONTAINER/etc/machine-id
sudo rm $CONTAINER/etc/resolv.conf
echo "nameserver 8.8.8.8" > $CONTAINER/etc/resolv.conf

echo "Set the root password for root on the container."
echo "By running:"
echo "  sudo systemd-nspawn -D /mnt/pd0/container"
echo "  passwd"
