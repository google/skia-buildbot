#!/bin/bash

# This script will check out the code for the libimobiledevice family
# of tools and build them into an out/ dir. Use package.sh to roll them
# into a .deb and install them before testing, or you'll have linker
# headaches.

set -x -e

PREFIX=`pwd`/out
mkdir -p ${PREFIX}

# Check out a git repo at a specific revision under the current working directory.
check_out()
{
    REPO="$1"
    REV="$2"
    DIR=`echo "$REPO" | tr / $'\n' | tail -1 | sed -e "s/\.git$//"`

    git clone "$REPO"
    cd "$DIR"
    git checkout "$REV"
    cd ..
}

check_out https://github.com/libimobiledevice/libplist.git cf7a3f3d7c06b197ee71c9f97eb9aa05f26d63b5
check_out https://github.com/libimobiledevice/libimobiledevice-glue.git 7c37434360f1c49975c286566efc3f0c935a84ef
check_out https://github.com/libimobiledevice/libusbmuxd.git 2ec5354a6ff2ba5e2740eabe7402186f29294f79
check_out https://github.com/skia-dev/libimobiledevice.git bf5f66f7216b7147e36629cb0f698a41053bb854
check_out https://github.com/libimobiledevice/ifuse.git 14839dcda4ec8c86f11372665c853dc4a294fa72
check_out https://github.com/libimobiledevice/ideviceinstaller.git d5c37d657969a6c71ff965a3f17004a844449879
check_out https://github.com/libimobiledevice/usbmuxd.git e3a3180b9b380ce9092ee0d7b8e9d82d66b1c261

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
