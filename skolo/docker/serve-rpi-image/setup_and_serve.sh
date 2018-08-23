#!/usr/bin/env bash

# Sets up the image (using $IMAGE_PATH to configure stage or prod), and then
# call into the base image's entry point to start serving NFS.

# The reason we have to do it this way is that we can't mount an image during build.
# Mounting the image at runtime is allowed (when run with --privileged), so we do
# that and then defer to entrypoint.sh which is from https://github.com/ehough/docker-nfs-server
# and takes care of the actual NFS serving

# Set the exports
echo "$IMAGE_PATH 192.168.1.0/24(ro,no_root_squash,sync,no_subtree_check,fsid=0)" > /etc/exports

mkdir -p $IMAGE_PATH
mount /opt/rpi.img $IMAGE_PATH -o ro,norecovery,offset=67108864,sizelimit=2367684608,noauto
/usr/local/bin/entrypoint.sh
