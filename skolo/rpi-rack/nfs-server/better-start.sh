#!/bin/bash

# ./umount_sdcard.sh

set -e -x

# ./mount_sdcard.sh

docker run -it \
  --net=host \
  --privileged \
  nfs4-image

# add    to expose it on the network
