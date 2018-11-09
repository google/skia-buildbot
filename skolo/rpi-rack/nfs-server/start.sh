#!/bin/bash
set -e -x

docker run -it \
  --net=host \
  --privileged  \
  -v /home/chrome-bot/sd_card:/exported \
  nfs4-image


# add    to expose it on the network
