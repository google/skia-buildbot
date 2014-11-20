#!/bin/bash
#
# Updates the code running on skiamonitor.com.
#
set -x

source vm_config.sh

gcutil --project=$PROJECT_ID ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
  "cd ~/buildbot;" \
  "git pull;" \
  "cd compute_engine_scripts/monitoring;" \
  "./setup.sh"
