#!/bin/bash

# This script will check out the code for the libimobiledevice family
# of tools and build them.

set -x -e

PREFIX=`pwd`/out
mkdir -p ${PREFIX}

rm -rf out libplist libusbmuxd usbmuxd libimobiledevice ifuse ideviceinstaller

git clone https://github.com/libimobiledevice/libplist.git
git clone https://github.com/libimobiledevice/usbmuxd.git
git clone https://github.com/libimobiledevice/libusbmuxd.git
git clone https://github.com/libimobiledevice/libimobiledevice.git
git clone https://github.com/libimobiledevice/ifuse.git
git clone https://github.com/libimobiledevice/ideviceinstaller.git

# Make sure the libraries below are found.
export PKG_CONFIG_PATH="${PREFIX}/lib/pkgconfig"

# Build and install in order of dependencies.
cd libplist && ./autogen.sh --prefix=$PREFIX --without-cython && make && make install && cd ..
cd libusbmuxd && ./autogen.sh --prefix=$PREFIX && make && make install && cd ..
cd libimobiledevice && ./autogen.sh --prefix=$PREFIX --without-cython && make && make install && cd ..
cd ifuse && ./autogen.sh --prefix=$PREFIX && make && make install && cd ..

# Patch a specific commit of ideviceinstaller so it can be compile on RPi Stretch
# The reason might be 32-bit vs 64-bit architectures or the fact that RPi Stretch 
# uses gcc 6 by default while Debian uses gcc 7. 
cd ideviceinstaller
git checkout f7988de8279051f3d2d7973b8d7f2116aa5d9317
git am ../patches/ideviceinstaller.patch
./autogen.sh --prefix=$PREFIX
make
make install 
cd ..

cd usbmuxd
./autogen.sh --prefix=$PREFIX \
              --with-udevrulesdir=$PREFIX/udev-rules \
              --with-systemdsystemunitdir=$PREFIX/systemd
make && make install && cd ..
