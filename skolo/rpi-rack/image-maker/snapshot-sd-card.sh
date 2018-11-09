#!/bin/bash 

sudo umount /mnt/boot
sudo umount /mnt/root
sudo dd bs=4M if=/dev/mmcblk0 | gzip > rpi-image`date +%d%m%y`.img.gz
sudo sync

