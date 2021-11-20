#!/bin/bash

# Get the packaged libs out of the way so the linker doesn't grab the wrong ones:
sudo apt remove ideviceinstaller ifuse libimobiledevice-utils libimobiledevice6 libplist3 libusbmuxd6 usbmuxd

# Add build deps:
sudo apt-get -y install build-essential checkinstall git autoconf automake libtool-bin pkgconf libssl-dev fuse libfuse-dev libzip-dev libusb-1.0-0-dev
# Without pkgconf, the configure scripts complain of syntax errors.
