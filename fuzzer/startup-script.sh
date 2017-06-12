#! /bin/bash

set -e
set -x

AFL_VERSION="2.35b"
# We need clang set as our c++ builder to build afl-clang
export CC=/usr/bin/clang CXX=/usr/bin/clang++

# Download and install afl-fuzz
sudo rm -rf /mnt/ssd0/afl
sudo mkdir /mnt/ssd0/afl
sudo chmod 777 /mnt/ssd0/afl
wget 'https://storage.googleapis.com/skia-fuzzer/afl-mirror/afl-'$AFL_VERSION'.tgz' -O /tmp/afl.tgz
tar -C /mnt/ssd0/afl/ -zxf /tmp/afl.tgz --strip=1 "afl-"$AFL_VERSION
cd /mnt/ssd0/afl/
make
# build afl-clang-fast
cd /mnt/ssd0/afl/llvm_mode/
make

# Download and install depot_tools to /mnt/ssd0/depot_tools
git clone 'https://chromium.googlesource.com/chromium/tools/depot_tools.git' /mnt/ssd0/depot_tools
sudo chmod 777 /mnt/ssd0/depot_tools

# Fix afl-fuzz's requirement on core
sudo sh -c "echo 'core' >/proc/sys/kernel/core_pattern"
