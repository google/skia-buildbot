#!/bin/bash

set -e
set -x

AFL_VERSION="2.51b"
# We need clang set as our c++ builder to build afl-clang
export CC=/usr/bin/clang CXX=/usr/bin/clang++

# Download and install afl-fuzz
sudo rm -rf /mnt/pd0/afl
sudo rm -f /tmp/afl.tgz
sudo mkdir /mnt/pd0/afl
sudo chmod 777 /mnt/pd0/afl

set +e
# wget sometimes fails right after boot-up. It may be due to clocks changing, or CRNG
# initialization which throws off TLS connections. We see errors like:
# GnuTLS: The TLS connection was non-properly terminated.
# Retrying the get gets around these failures.
for i in {1..5}; do
    wget --retry-connrefused --waitretry=1 --read-timeout=10 --timeout=10 --tries 5 \
        'https://storage.googleapis.com/skia-fuzzer/afl-mirror/afl-'$AFL_VERSION'.tgz' -O /tmp/afl.tgz
    if [ $? = 0 ]; then break; fi; # check return value, break if successful (0)
    sleep 1s;
done;
set -e

tar -C /mnt/pd0/afl/ -zxf /tmp/afl.tgz --strip=1 "afl-"$AFL_VERSION
cd /mnt/pd0/afl/
make
# build afl-clang-fast
cd /mnt/pd0/afl/llvm_mode/
make

# Fix afl-fuzz's requirement on core
sudo sh -c "echo 'core' >/proc/sys/kernel/core_pattern"

# Download and install depot_tools to /mnt/pd0/depot_tools
git clone 'https://chromium.googlesource.com/chromium/tools/depot_tools.git' /mnt/pd0/depot_tools
sudo chmod 777 /mnt/pd0/depot_tools
