#!/bin/bash

# Presumes that an RPi image is mounted to the local machine and writes the
# updated information we need for that machine, which is just setting the
# hostname and copying over authorized_keys for root. All the rest of the configuration
# is done via Ansible. See README.md.

# Record the directory of this file.
REL=$(dirname "$0")

if [ "$(uname -s)" = Darwin ]; then
    DEFAULT_MOUNT=/Volumes/RASPIROOT
else
    DEFAULT_MOUNT=/media/${USER}/RASPIROOT
fi

# Check argument count is valid.
if [ $# == 0 ]; then
    echo "$0 NAME [DIR]"
    echo ""
    echo "Where:"
    echo "    DIR is the optional directory where the SD card is mounted. Defaults to '$DEFAULT_MOUNT'."
    echo "    NAME is the desired hostname of the RPi, such as 'skia-rpi2-rack4-shelf2-025'."
    exit 1
fi

HOSTNAME=$1;
MOUNT=$DEFAULT_MOUNT
if [ $# == 2 ]; then
    MOUNT=$2
fi

if [ ! -f "$MOUNT/etc/hostname" ]; then
    echo "Unable to find SD card with image mounted at $MOUNT."
    exit 1
fi

sudo su root -c "./configure-image-impl.sh $HOSTNAME $MOUNT"
