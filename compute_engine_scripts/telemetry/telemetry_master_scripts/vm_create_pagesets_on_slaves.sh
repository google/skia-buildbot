#
# Starts the telemetry_slave_scripts/vm_capture_archives.sh script on all
# slaves.
#
# The script should be run from the skia-telemetry-master GCE instance's
# /home/default/skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_master_scripts
# directory.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source ../vm_config.sh

# Update buildbot.
gclient sync

NUM_WEBPAGES_PER_SLAVE=$(($NUM_WEBPAGES/$NUM_SLAVES))
START=1
for SLAVE_NUM in $(seq 1 $NUM_SLAVES); do
  END=$(expr $START + $NUM_WEBPAGES_PER_SLAVE - 1)
  CMD="bash vm_create_pagesets.sh $SLAVE_NUM $START"
  START=$(expr $END + 1)
  ssh -f -X -o UserKnownHostsFile=/dev/null -o CheckHostIP=no \
    -o StrictHostKeyChecking=no -i /home/default/.ssh/google_compute_engine \
    -A -p 22 default@108.170.222.$SLAVE_NUM -- "source .bashrc; cd skia-repo/buildbot/compute_engine_scripts/telemetry/telemetry_slave_scripts; $CMD > /tmp/create_pagesets_output.txt 2>&1"
done

