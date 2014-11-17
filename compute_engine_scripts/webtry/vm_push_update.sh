#!/bin/bash
#
# Updates the code running on skfiddle.com
#
set -x

source ./vm_config.sh

gcutil --project=$PROJECT_ID ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
  "cd ~/buildbot/webtry/setup;" \
  "git pull;" \
  "./webtry_setup.sh"
