#! /bin/bash
# Install clang/llvm
wget -O - http://llvm.org/apt/llvm-snapshot.gpg.key|sudo apt-key add -
sudo apt-get update
sudo apt-get install clang-3.6 lldb-3.6 make build-essential libfontconfig1-dev libfreetype6-dev libgif-dev libpng12-dev libqt4-dev ninja-build python-dev python-imaging libosmesa-dev -y
# Afl-fuzz can't find the clang-3.6 aliases, so make the standard /usr/bin/clang
sudo ln /usr/bin/clang-3.6 /usr/bin/clang
sudo ln /usr/bin/clang++-3.6 /usr/bin/clang++
# Make symbolizer easier to find
sudo ln /usr/bin/llvm-symbolizer-3.6 /usr/bin/llvm-symbolizer
