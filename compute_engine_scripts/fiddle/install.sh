#! /bin/bash
set -x

/tmp/format_and_mount.sh skia-fiddle

# The same set of packages need to be installed both on the instance and within the container.
PACKAGES="systemd-container git debootstrap build-essential libosmesa6-dev libfreetype6-dev libfontconfig1-dev libpng-dev libgif-dev libqt4-dev mesa-common-dev ffmpeg libglu1-mesa"
sudo apt-get update
sudo apt-get --assume-yes upgrade
sudo apt-get --assume-yes install ${PACKAGES}

# Install depot_tools
mkdir --parents /mnt/pd0/fiddle
cd /mnt/pd0/fiddle
git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git
echo -e "\nexport PATH=/mnt/pd0/fiddle/depot_tools:\$PATH" >> ~/.bashrc

# Build the containter
CONTAINER=/mnt/pd0/container
# Note that the name 'yakkety' needs to match the release name of the distro
# image, i.e. 16.10, which is specified when creating the pushable snapshot.
sudo debootstrap --arch=amd64 --components main,restricted,universe,multiverse --include=${PACKAGES// /,} yakkety /mnt/pd0/container

sudo mkdir -p /mnt/pd0/container/mnt/pd0/fiddle
sudo rm $CONTAINER/etc/machine-id
sudo rm $CONTAINER/etc/resolv.conf

cd /tmp
cat >initcontainer.sh <<EOL
sudo echo "nameserver 8.8.8.8" > /etc/resolv.conf
passwd
sudo ln -s /usr/lib/x86_64-linux-gnu/mesa/libGL.so.1 /usr/lib/libGL.so
sudo ln -s /usr/lib/x86_64-linux-gnu/libGLU.so.1 /usr/lib/libGLU.so
EOL

chmod +x initcontainer.sh
sudo cp initcontainer.sh /mnt/pd0/container/mnt/pd0/fiddle
sudo systemd-nspawn -n -D /mnt/pd0/container /mnt/pd0/fiddle/initcontainer.sh
