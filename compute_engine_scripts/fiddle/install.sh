#! /bin/bash
set -x

export TERM=xterm

/tmp/format_and_mount.sh skia-fiddle

# The same set of packages need to be installed both on the instance and within the container.
PACKAGES="git debootstrap build-essential libosmesa6-dev libfreetype6-dev libfontconfig1-dev libpng-dev libgif-dev libqt4-dev mesa-common-dev ffmpeg libglu1-mesa clinfo nvidia-driver nvidia-cuda-dev libegl1-mesa-dev libgles2-mesa-dev"
sudo apt-get update
sudo apt-get --assume-yes upgrade
sudo apt-get --assume-yes install ${PACKAGES}

# Install depot_tools
mkdir --parents /mnt/pd0/fiddle
cd /mnt/pd0/fiddle
git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git
echo -e "\nexport PATH=/mnt/pd0/fiddle/depot_tools:\$PATH" >> ~/.bashrc

nvidia-smi
