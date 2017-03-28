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
cd libimobiledevice && ./autogen.sh --prefix=$PREFIX && make && make install && cd ..
cd ifuse && ./autogen.sh --prefix=$PREFIX && make && make install && cd ..
cd ideviceinstaller && ./autogen.sh --prefix=$PREFIX && make && make install && cd ..

cd usbmuxd
./autogen.sh --prefix=$PREFIX \
              --with-udevrulesdir=$PREFIX/udev-rules \
              --with-systemdsystemunitdir=$PREFIX/systemd
make && make install && cd ..
