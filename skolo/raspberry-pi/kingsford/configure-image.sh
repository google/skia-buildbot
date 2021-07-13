#!/bin/bash

# Presumes that an RPi image is mounted to the local machine and writes the
# updated information we need for that machine, which is just setting the
# hostname and copying over authorized_keys for root.

# Record the directory of this file.
REL=$(dirname "$0")

# Check argument count is valid.
if [ $# == 0 ]; then
    echo "$0 <test-machine-hostname>"
    echo ""
    echo "Such as skia-rpi2-rack4-shelf2-025"
    exit 1
fi

HOSTNAME=$1;
MOUNT=/media/${USER}/RASPIROOT

sudo su -c "./configure-image-impl.sh $HOSTNAME $MOUNT" root

