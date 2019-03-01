#!/bin/bash

set -x -e

TMP_ROOT=./tmproot
TMP_BOOT=./tmpboot

if [ ! -f $1 ]; then
    echo "File '$1' not found!"
    exit 1
fi

mkdir -p ${TMP_ROOT}
mkdir -p ${TMP_BOOT}

# mount the root partition and fix anything that needs to be fixed.
sudo mount -o offset=50331648 -t ext4 $1 ${TMP_ROOT}
sudo cp fstab ${TMP_ROOT}/etc/fstab
sudo cp interfaces ${TMP_ROOT}/etc/network/interfaces
sudo rm -f ${TMP_ROOT}/etc/hostname
sudo umount ${TMP_ROOT}

# mount the boot partition and fix anything necessary.
sudo mount -o offset=4194304,sizelimit=45297664 -t vfat $1 ${TMP_BOOT}
if [ -f ${TMP_BOOT}/disable-bootcode.bin ]; then
   sudo mv ${TMP_BOOT}/disable-bootcode.bin ${TMP_BOOT}/bootcode.bin
fi
sudo umount ${TMP_BOOT}

# Shrink the image.
sudo ./pishrink.sh -s $1
