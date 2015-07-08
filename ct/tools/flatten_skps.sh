#!/bin/bash

# This script flattens the SKPs in the specified directory and uploads them to
# the specified Google Storage directory.

unflattened_skps_location=$1
gsutil_destination=$2

# Make tools.
cd /b/skia-repo/trunk
/b/depot_tools/gclient sync
make tools

# Make temporary local directory to store SKPs.
flattened_skps_location=/b/storage/skps/flattened
rm -rf $flattened_skps_location
mkdir -p $flattened_skps_location

# Run the flatten tool.
skps=$unflattened_skps_location/*.skp
for s in $skps; do
  skpname=$(basename "$s")
  out/Debug/flatten $s $flattened_skps_location/$skpname
done

# Upload flattened SKPs to Google Storage.
gsutil -m cp $flattened_skps_location/*.skp $gsutil_destination/
