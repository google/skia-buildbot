#!/bin/bash
#
# Set up the Skia rebaseline_server VM instance(s).
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: epoger@google.com (Elliot Poger)

source vm_config.sh
source vm_setup_utils.sh

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_REBASELINESERVER[@]}; do
  if [ ! -f $REQUIRED_FILE ];
  then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done

for VM in $VM_REBASELINESERVER_NAMES; do
  VM_COMPLETE_NAME="${VM_NAME_BASE}-${VM}-${ZONE_TAG}"

  echo """

================================================
Starting setup of ${VM_COMPLETE_NAME}.....
================================================

"""

  checkout_depot_tools

  echo
  echo "===== Copying over required rebaseline_server files. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
    "mkdir /home/$PROJECT_USER/rebaseline_server" \
    || echo "Failed to set up launch-on-reboot!"
  for REQUIRED_FILE in ${REQUIRED_FILES_FOR_REBASELINESERVER[@]}; do
    $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
      $REQUIRED_FILE /home/$PROJECT_USER/rebaseline_server/
  done

  echo
  echo "===== Installing crontab ======"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
    "chmod a+x /home/$PROJECT_USER/rebaseline_server/kick-rebaseline-server.sh && " \
    "crontab /home/$PROJECT_USER/rebaseline_server/rebaseline-server-crontab" \
    || echo "Failed to install crontab!"
  echo

done
