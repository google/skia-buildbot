#! /bin/bash
#
# Formats and sets up fstab rules for /mnt/pd0
#
# The first argument is expected to be the partial name of the disk
# a la google-skia-[DISK_NAME]-data
set -x

DISK_NAME="google-skia-$1-data"

echo "Formatting and setting up fstab rules for "$DISK_NAME
# Mount data disk
sudo mkdir -p /mnt/pd0
sudo /tmp/safe_format_and_mount "/dev/disk/by-id/"$DISK_NAME /mnt/pd0
sudo chmod 777 /mnt/pd0

# Add mounting instructions to fstab so it remounts on reboot.
echo "/dev/disk/by-id/"$DISK_NAME" /mnt/pd0 ext4 discard,defaults 1 1" | sudo tee -a /etc/fstab