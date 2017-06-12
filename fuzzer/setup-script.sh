#! /bin/bash

set -e

# Install clang/llvm
wget -O - http://llvm.org/apt/llvm-snapshot.gpg.key|sudo apt-key add -
sudo apt-get update
sudo apt-get install clang-3.8 lldb-3.8 make build-essential libfontconfig1-dev libfreetype6-dev libgif-dev libpng12-dev libqt4-dev ninja-build python-dev python-imaging libosmesa-dev -y
# Afl-fuzz can't find the clang-3.8 aliases, so make the standard /usr/bin/clang
sudo ln /usr/bin/clang-3.8 /usr/bin/clang
sudo ln /usr/bin/clang++-3.8 /usr/bin/clang++
sudo ln /usr/bin/llvm-config-3.8 /usr/bin/llvm-config
# Make symbolizer easier to find
sudo ln /usr/bin/llvm-symbolizer-3.8 /usr/bin/llvm-symbolizer

# Mount data disk
sudo mkdir -p /mnt/ssd0
sudo mkfs.ext4 -F /dev/disk/by-id/google-local-ssd-0
sudo mount -o discard,defaults /dev/disk/by-id/google-local-ssd-0 /mnt/ssd0
sudo chmod 777 /mnt/ssd0

# Add mounting instructions to fstab so it remounts on reboot.
echo '/dev/disk/by-id/google-local-ssd-0 /mnt/ssd0 ext4 discard,defaults 1 1' | sudo tee -a /etc/fstab
