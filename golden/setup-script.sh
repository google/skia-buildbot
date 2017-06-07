#! /bin/bash

set -e
set -x

DATADISK_ROOT="/mnt/pd0"

if [ ! -d "$DATADISK_ROOT/data" ]; then
  mkdir -p "$DATADISK_ROOT/data"
  chown default:default "$DATADISK_ROOT/data"
  chmod 755 "$DATADISK_ROOT/data"
  git clone https://skia.googlesource.com/skia/ "$DATADISK_ROOT/data/skia"
  chown -R default:default "$DATADISK_ROOT/data/skia"
fi
