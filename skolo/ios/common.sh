

# build_install will check out the code for the libimobiledevice family
# of tools and build them. A directory can be provided as an argument.
# In that case it will install it at the provided location.
build_install() {
  PREFIX=$1
  if [ -z "$1" ]; then
    PREFIX="/usr/local/bin"
  else
    PREFIX=$1
  fi

  rm -rf out libplist libusbmuxd usbmuxd libimobiledevice ifuse ideviceinstaller

  git clone https://github.com/libimobiledevice/libplist.git
  git clone https://github.com/libimobiledevice/usbmuxd.git
  git clone https://github.com/libimobiledevice/libusbmuxd.git
  git clone https://github.com/libimobiledevice/libimobiledevice.git
  git clone https://github.com/libimobiledevice/ifuse.git
  git clone https://github.com/libimobiledevice/ideviceinstaller.git


  cd libplist && ./autogen.sh --prefix=$PREFIX --without-cython && make
  if [ -n "$1" ]; then
    # sudo make install
    make install
  fi
  cd ..

  export LDFLAGS="-L${PREFIX}/lib"
  export CPPFLAGS="-I${PREFIX}/include"
  export LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:${PREFIX}/lib"
  export PKG_CONFIG_PATH="${PREFIX}/lib/pkgconfig"

  cd libusbmuxd && ./autogen.sh --prefix=$PREFIX && make
  if [ -n "$1" ]; then
    # sudo make install
    make install
  fi
  cd ..

  cd libimobiledevice && ./autogen.sh --prefix=$PREFIX && make
  if [ -n "$1" ]; then
    # sudo make install
    make install
  fi
  cd ..

  cd ifuse && ./autogen.sh --prefix=$PREFIX && make
  if [ -n "$1" ]; then
    # sudo make install
    make install
  fi
  cd ..

  cd ideviceinstaller && ./autogen.sh --prefix=$PREFIX && make
  if [ -n "$1" ]; then
    # sudo make install
    make install
  fi
  cd ..

  cd usbmuxd
  ./autogen.sh --prefix=$PREFIX \
               --with-udevrulesdir=$PREFIX/udev-rules \
               --with-systemdsystemunitdir=$PREFIX/systemd
  make
  if [ -n "$1" ]; then
    # sudo make install
    make install
  fi
  cd ..
}
