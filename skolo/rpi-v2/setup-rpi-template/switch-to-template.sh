#!/bin/bash 

DIS_BOOT_FILE=/mnt/boot/disable-bootcode.bin
BOOT_FILE=/mnt/boot/bootcode.bin

sudo mount /mnt/boot
if [ -f $DIS_BOOT_FILE ]; then
   sudo mv ${DIS_BOOT_FILE} ${BOOT_FILE}
fi
