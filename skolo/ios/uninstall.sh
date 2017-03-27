#!/bin/bash

set -e 
# set -e -x 

cd libplist && sudo make uninstall-recursive && cd ..
cd libusbmuxd && sudo make uninstall-recursive && cd ..
cd libimobiledevice && sudo make uninstall-recursive && cd ..
cd ifuse && sudo make uninstall-recursive && cd ..
cd ideviceinstaller && sudo make uninstall-recursive && cd ..
cd usbmuxd && sudo make uninstall-recursive && cd ..

