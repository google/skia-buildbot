#!/bin/bash
#
# Setup all the software running on skiamonitor.com.
#

source ../buildbots/vm_config.sh

# Basically SSH in, clone this repo and jump to a shell script in the repo.

gcutil --project=$PROJECT_ID ssh --ssh_user=$PROJECT_USER skia-monitoring-$ZONE_TAG \
  "cd ~/buildbot;" \
  "git clone https://skia.googlesource.com/buildbot;" \
  "cd buildbot/compute_engine_scripts/monitoring;" \
  "./graphite_setup.sh"

echo "Make sure to 'set daemon 2' in /etc/monit/monitrc"
