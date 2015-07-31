#!/bin/bash

set -x

# link_redis_dir(SRC, TGT) replaces the SRC directory with a soft link to
# TGT, if SRC is not already a link.
function link_redis_dir {
  local SRC=$1
  local TGT=$2
  if [[ ! -L "$SRC" ]]; then
          rm -rf "$SRC"
          mkdir -p "$TGT"
          chown redis:redis "$TGT"
          ln -s "$TGT" "$SRC"
  fi
}

# Make sure the redis server is installed.
sudo apt-get update
sudo apt-get install -y redis-server

# Move the data and log directory of redis to the data disk.
REDIS_DATA_DIR="/var/lib/redis"
REDIS_LOG_DIR="/var/log/redis"
DATADISK_REDIS_DATA_DIR="/mnt/pd0/redis-data"
DATADISK_REDIS_LOG_DIR="/mnt/pd0/redis-log"

link_redis_dir $REDIS_DATA_DIR $DATADISK_REDIS_DATA_DIR
link_redis_dir $REDIS_LOG_DIR $DATADISK_REDIS_LOG_DIR

sudo systemctl restart skiacorrectness.service ingest.service
