#!/bin/bash
#
# Update the code running on skiamonitor.com.
#

source ../buildbots/vm_config.sh

gcutil --project=$PROJECT_ID ssh --ssh_user=$PROJECT_USER skia-monitoring \
  "cd ~/buildbot;" \
  "git pull;" \
  "cd compute_engine_scripts/monitoring;" \
  "./setup.sh"
