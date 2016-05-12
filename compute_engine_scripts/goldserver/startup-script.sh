#! /bin/bash

set -x

FIRST_BOOT_FILE="/var/lib/initial_setup_complete"
DATADISK_ROOT="/mnt/pd0"

if [ ! -e $FIRST_BOOT_FILE ]; then
  if [ ! -d "$DATADISK_ROOT/data" ]; then
    mkdir -p "$DATADISK_ROOT/data"
    chown default:default "$DATADISK_ROOT/data"
    chmod 755 "$DATADISK_ROOT/data"
    git clone https://skia.googlesource.com/skia/ "$DATADISK_ROOT/data/skia"
    chown -R default:default "$DATADISK_ROOT/data/skia"
  fi

  sudo touch $FIRST_BOOT_FILE
  echo "First boot finished. Created first boot file at $FIRST_BOOT_FILE"
else
  echo "First boot file found skipped setup procedure."
fi
