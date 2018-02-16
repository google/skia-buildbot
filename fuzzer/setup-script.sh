#! /bin/bash

set -e

# Install clang/llvm
wget -O - http://llvm.org/apt/llvm-snapshot.gpg.key|sudo apt-key add -
sudo apt-get update
sudo apt-get install clang-3.8 lldb-3.8 make build-essential libfontconfig1-dev libfreetype6-dev libgif-dev libqt4-dev ninja-build python-dev python-imaging libosmesa6-dev libc++-dev -y
# Afl-fuzz can't find the clang-3.8 aliases, so make the standard /usr/bin/clang
sudo ln /usr/bin/clang-3.8 /usr/bin/clang
sudo ln /usr/bin/clang++-3.8 /usr/bin/clang++
sudo ln /usr/bin/llvm-config-3.8 /usr/bin/llvm-config
# Make symbolizer easier to find
sudo ln /usr/bin/llvm-symbolizer-3.8 /usr/bin/llvm-symbolizer

# Set up data disk
sudo chmod 777 /mnt/pd0
