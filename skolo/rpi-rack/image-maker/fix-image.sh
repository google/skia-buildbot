#!/bin/bash

set -x

if [ ! -f $1 ]; then
    echo "File '$1' not found!"
    exit 1
fi

mkdir -p ./tmpmnt

# mount the image and fix anything that needs to be fixed.
sudo mount -o offset=50331648 -t ext4 $1 ./tmpmnt
sudo cp fstab ./tmpmnt/etc/fstab
sudo umount ./tmpmnt

# Shrink the image.
sudo ./pishrink.sh -s $1
