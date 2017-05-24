#! /bin/bash
#
# Formats and sets up fstab rules for the mounted path
#
# The first argument is expected to be the partial name of the disk
# a la google-[DISK_NAME]
# The optional second argument is expected to be the path where the disk is to
# be mounted. If not specified then "/mnt/pd0" is used.
set -x -e

DISK_NAME="google-$1"

MOUNTED_PATH="/mnt/pd0"
if [ ! -z "$2" ]; then
    MOUNTED_PATH="$2"
fi

echo "Formatting and setting up fstab rules for "$DISK_NAME
# Mount data disk
sudo mkdir -p $MOUNTED_PATH
sudo /tmp/safe_format_and_mount "/dev/disk/by-id/"$DISK_NAME $MOUNTED_PATH
sudo chmod 777 $MOUNTED_PATH

# Add mounting instructions to fstab so it remounts on reboot.
echo "/dev/disk/by-id/$DISK_NAME $MOUNTED_PATH ext4 discard,defaults 1 1" | sudo tee -a /etc/fstab
