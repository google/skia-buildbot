#!/bin/bash 

# Make sure the template is a bootable state.
./switch-to-template.sh

# Unmount the template sdcard and create a snapshot of them.
sudo umount /mnt/boot
sudo umount /mnt/root
sudo dd bs=4M if=/dev/mmcblk0 | gzip > rpi-image`date +%d%m%y`.img.gz
sudo sync
