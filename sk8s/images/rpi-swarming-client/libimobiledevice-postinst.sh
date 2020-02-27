#!/bin/bash

# add the usb group and user if they don't exist.
if ! getent group plugdev >/dev/null; then
        echo "Adding group plugdev"
        addgroup --system plugdev
fi
if ! getent passwd usbmux >/dev/null; then
        echo "Adding user usbmux"
        adduser --system --ingroup plugdev --no-create-home --gecos "usbmux daemon" usbmux
fi
/bin/systemctl enable usbmuxd
