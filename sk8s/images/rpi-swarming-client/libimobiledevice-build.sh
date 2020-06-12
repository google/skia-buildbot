#!/bin/bash

# This script will check out the code for the libimobiledevice family
# of tools and build them.

set -x -e

PREFIX=`pwd`/out
mkdir -p ${PREFIX}

git clone https://github.com/libimobiledevice/libplist.git
git clone https://github.com/libimobiledevice/usbmuxd.git
git clone https://github.com/libimobiledevice/libusbmuxd.git
git clone https://github.com/libimobiledevice/libimobiledevice.git
git clone https://github.com/libimobiledevice/ifuse.git
git clone https://github.com/libimobiledevice/ideviceinstaller.git

# Make sure the libraries below are found.
export PKG_CONFIG_PATH="${PREFIX}/lib/pkgconfig"
export CPPFLAGS="-I${PREFIX}/include"
export LDFLAGS="-L${PREFIX}/lib"

# Build and install in order of dependencies.
cd libplist
./autogen.sh --prefix=$PREFIX --without-cython
make
make install
cd ..

cd libusbmuxd
./autogen.sh --prefix=$PREFIX
make
make install
cd ..

cd libimobiledevice
# git requires user config to run 'merge'
git config --global user.email "skiabot@google.com"
git config --global user.name "Skia Infra Docker Build"
# Apply patch for idevicedebug debug output.
# https://github.com/libimobiledevice/libimobiledevice/pull/716
git fetch origin pull/716/head
git merge --no-edit FETCH_HEAD
# Apply patch for iOS 13.
# https://github.com/libimobiledevice/libimobiledevice/pull/860
git fetch origin pull/860/head
git merge --no-edit FETCH_HEAD
# Apply patch to fix handling of replies.
# https://github.com/libimobiledevice/libimobiledevice/pull/914
git fetch origin pull/914/head
git merge --no-edit FETCH_HEAD
# Apply patch to enable debugserver debug logging to syslog.
# https://github.com/libimobiledevice/libimobiledevice/pull/646
# Pull request no longer applies cleanly; pull from skia-dev mirror instead.
#git fetch origin pull/646/head
#git merge --no-edit FETCH_HEAD
git remote add skiadev https://github.com/skia-dev/libimobiledevice.git
git fetch skiadev
git cherry-pick fc4e271bb57a48fdfaf14cd68e8a3e03f5e237aa
./autogen.sh --prefix=$PREFIX --without-cython --enable-debug-code
make
make install
cd ..

cd ifuse
./autogen.sh --prefix=$PREFIX
make
make install
cd ..

cd ideviceinstaller
./autogen.sh --prefix=$PREFIX
make
make install 
cd ..

cd usbmuxd
./autogen.sh --prefix=$PREFIX \
  --with-udevrulesdir=$PREFIX/udev-rules \
  --with-systemdsystemunitdir=$PREFIX/systemd
make
make install
cd ..

sed -i "s+${PREFIX}/sbin+/sbin+g" ${PREFIX}/systemd/usbmuxd.service
sed -i "s+${PREFIX}/var/run+/var/run+g"     ${PREFIX}/systemd/usbmuxd.service
sed -i "s+${PREFIX}/sbin+/sbin+g" ${PREFIX}/udev-rules/*.rules
