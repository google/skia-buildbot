#!/bin/bash

# This script will check out the code for the libimobiledevice family
# of tools and build them.

set -x -e

PREFIX=`pwd`/out
mkdir -p ${PREFIX}

rm -rf out libplist libusbmuxd usbmuxd libimobiledevice ifuse ideviceinstaller #idevice-app-runner

git clone https://github.com/libimobiledevice/libplist.git
git clone https://github.com/libimobiledevice/usbmuxd.git
git clone https://github.com/libimobiledevice/libusbmuxd.git
git clone https://github.com/libimobiledevice/libimobiledevice.git
git clone https://github.com/libimobiledevice/ifuse.git
git clone https://github.com/libimobiledevice/ideviceinstaller.git
#git clone https://github.com/storoj/idevice-app-runner.git

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
# Apply patch for idevicedebug debug output.
git fetch origin pull/716/head
git cherry-pick FETCH_HEAD
# Apply patch for iOS 13.
git fetch origin pull/860/head
git cherry-pick FETCH_HEAD
# Apply patch to process exit status.
git remote add skiadev https://github.com/skia-dev/libimobiledevice.git
git fetch skiadev 06079a6a12d9b2e27628fe0319f3c00c554a39c5
git fetch skiadev 0a90bb9ab5525ad07afb852ad332f8ec41d3162a
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

#cd idevice-app-runner
#CPATH=$PREFIX/include LIBRARY_PATH=$PREFIX/lib make
#DESTDIR=$PREFIX make install
#cd ..
