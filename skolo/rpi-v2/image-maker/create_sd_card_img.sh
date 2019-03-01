#!/bin/bash

set -e -x

# create the sd card image
dd if=/dev/zero of=sd_card.img bs=1M count=3500

# create the partition table
sfdisk sd_card.img < partition.j2

# Format the partitions
sudo losetup -P /dev/loop0 sd_card.img
sudo fdisk -l /dev/loop0
sudo mkfs.fat /dev/loop0p1 -n BOOT
sudo mkfs.ext4 /dev/loop0p3 -L B
sudo mkfs.ext4 /dev/loop0p5 -L VAR
sudo mkfs.ext4 /dev/loop0p6 -L TMP
sudo mkfs.ext4 /dev/loop0p7 -L HOME

# mount the target partion
mkdir -p ./cardmnt/boot; sudo mount /dev/loop0p1 ./cardmnt/boot
mkdir -p ./cardmnt/var; sudo mount /dev/loop0p5 ./cardmnt/var
mkdir -p ./cardmnt/home; sudo mount /dev/loop0p7 ./cardmnt/home

# mount the source partions
sudo losetup -P /dev/loop1 rpi.img

mkdir -p ./imgmnt/boot; sudo mount /dev/loop1p1 ./imgmnt/boot
mkdir -p ./imgmnt/root; sudo mount /dev/loop1p2 ./imgmnt/root

# copy boot partion and the /var directory of the source image.
sudo cp -r ./imgmnt/boot/* ./cardmnt/boot/
sudo cp -r ./imgmnt/root/var/* ./cardmnt/var/

# set the ownership and .boto file of the home directory
sudo touch ./cardmnt/home/.boto
sudo chown -R 1001:1001 ./cardmnt/home

# make sure all data are synced
sudo sync
sleep 10

sudo umount ./cardmnt/boot
sudo umount ./cardmnt/var
sudo umount ./cardmnt/home
sudo umount ./imgmnt/boot
sudo umount ./imgmnt/root

sudo losetup -d /dev/loop1
sudo losetup -d /dev/loop0

