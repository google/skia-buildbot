#! /bin/bash
# Mount data disk
sudo mkdir -p /mnt/pd0
sudo /usr/share/google/safe_format_and_mount -m "mkfs.ext4 -F" /dev/disk/by-id/google-skia-fuzzer-data /mnt/pd0
sudo chmod 777 /mnt/pd0


AFL_VERSION="1.95b"

# Install clang/llvm
wget -O - http://llvm.org/apt/llvm-snapshot.gpg.key|sudo apt-key add -
sudo apt-get update
sudo apt-get install clang-3.6 lldb-3.6 make -y
# Afl-fuzz can't find the clang-3.6 aliases, so make the standard /usr/bin/clang
sudo ln /usr/bin/clang-3.6 /usr/bin/clang
sudo ln /usr/bin/clang++-3.6 /usr/bin/clang++
# We need clang set as our c++ builder to build afl-clang
export CC=/usr/bin/clang CXX=/usr/bin/clang++

# Download and install afl-fuzz
sudo mkdir /mnt/pd0/afl
sudo chmod 777 /mnt/pd0/afl
wget 'https://storage.googleapis.com/skia-fuzzer/afl-mirror/afl-'$AFL_VERSION'.tgz' -O /tmp/afl.tgz
tar -C /mnt/pd0/afl/ -zxf /tmp/afl.tgz --strip=1 "afl-"$AFL_VERSION
cd /mnt/pd0/afl/
make
