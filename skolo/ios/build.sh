#!/bin/bash

# This script will check out the code for the libimobiledevice family
# of tools and build them.

set -x -e

PREFIX=`pwd`/out
mkdir -p ${PREFIX}

rm -rf out libplist libusbmuxd usbmuxd libimobiledevice ifuse ideviceinstaller

git clone https://github.com/libimobiledevice/libplist.git
git clone https://github.com/libimobiledevice/libimobiledevice-glue.git
git clone https://github.com/libimobiledevice/usbmuxd.git
git clone https://github.com/libimobiledevice/libusbmuxd.git
git clone https://github.com/libimobiledevice/libimobiledevice.git
git clone https://github.com/libimobiledevice/ifuse.git
git clone https://github.com/libimobiledevice/ideviceinstaller.git

# Make sure the libraries below are found.
export CPPFLAGS="-I${PREFIX}/include"
export LDFLAGS="-L${PREFIX}/lib"

# Build and install in order of dependencies.
cd libplist
./autogen.sh --prefix=$PREFIX --without-cython
make
make install
cd ..

cd libimobiledevice-glue
./autogen.sh --prefix=$PREFIX
make
make install
cd ..

cd libusbmuxd
./autogen.sh --prefix=$PREFIX
make
make install
cd ..

cd libimobiledevice
# Apply patch to fix handling of replies.
# https://github.com/libimobiledevice/libimobiledevice/pull/914
# Unmerged as of 2021-11-08.
git fetch origin pull/914/head
git merge --no-edit FETCH_HEAD
# Apply patch to enable debugserver debug logging to syslog.
# https://github.com/libimobiledevice/libimobiledevice/pull/646
# Unmerged as of 2021-11-08.
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
