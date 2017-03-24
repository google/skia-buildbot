

# build_install will check out the code for the libimobiledevice family
# of tools and build them. A directory can be provided as an argument.
# In that case it will install it at the provided location.
build_install() {
  PREFIX=$1
  if [ -z "$1" ]; then
    PREFIX="/opt/local"
  else
    PREFIX=$1
  fi

  sudo apt-get -y install build-essential autoconf checkinstall libtool cython libzip-dev fuse libusb-1.0-0-dev  libfuse-dev libxml2-dev python2.7 python2.7-dev

  rm -rf libplist libusbmuxd usbmuxd libimobiledevice ifuse ideviceinstaller

  git clone https://github.com/libimobiledevice/libplist.git
  git clone https://github.com/libimobiledevice/usbmuxd.git
  git clone https://github.com/libimobiledevice/libusbmuxd.git
  git clone https://github.com/libimobiledevice/libimobiledevice.git
  git clone https://github.com/libimobiledevice/ifuse.git
  git clone https://github.com/libimobiledevice/ideviceinstaller.git

  cd libplist && ./autogen.sh --prefix=$PREFIX && make
  if [ -n "$1" ]; then
    sudo make install
  fi
  exit 1

  cd libusbmuxd && ./autogen.sh --prefix=$PREFIX && make
  if [ -n "$1" ]; then
    sudo make install
  fi
  cd ..

  cd libimobiledevice && ./autogen.sh --prefix=$PREFIX && make
  if [ -n "$1" ]; then
    sudo make install
  fi
  cd ..

  cd ifuse && ./autogen.sh --prefix=$PREFIX && make
  if [ -n "$1" ]; then
    sudo make install
  fi
  cd ..

  cd ideviceinstaller && ./autogen.sh --prefix=$PREFIX && make
  if [ -n "$1" ]; then
    sudo make install
  fi
  cd ..

  cd usbmuxd && ./autogen.sh --prefix=$PREFIX && make
  if [ -n "$1" ]; then
    sudo make install
  fi
  cd ..
}
