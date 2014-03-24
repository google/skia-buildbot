#!/bin/bash
#
# Setup all the master buildbot instances.
#
# Copyright 2012 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh
source vm_setup_utils.sh

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_MASTERS[@]}; do
  if [ ! -f $REQUIRED_FILE ];
  then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done

for VM in $VM_MASTER_NAMES; do
  VM_COMPLETE_NAME="${VM_NAME_BASE}-${VM}-${ZONE_TAG}"

  echo """

================================================
Starting setup of ${VM_COMPLETE_NAME}.....
================================================

"""

  checkout_depot_tools

  checkout_buildbot

  echo
  echo "===== Copying over required master files. ====="
  for REQUIRED_FILE in ${REQUIRED_FILES_FOR_MASTER[@]}; do
    $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
      $REQUIRED_FILE /home/$PROJECT_USER/$SKIA_REPO_DIR/buildbot/master/
  done

  echo
  echo "===== Setting up launch-on-reboot ======"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
    "cp $SKIA_REPO_DIR/buildbot/scripts/skiabot-master-start-on-boot.sh . && " \
    "chmod a+x skiabot-master-start-on-boot.sh && " \
    "echo \"@reboot /home/${PROJECT_USER}/skiabot-master-start-on-boot.sh ${SKIA_REPO_DIR}\" > reboot.txt && " \
    "crontab -u $PROJECT_USER reboot.txt && " \
    "rm reboot.txt" \
    || echo "Failed to set up launch-on-reboot!"
  echo

done

echo
echo "After the new master instances are turned on:"
echo "* Add the new IP Addresses (or flip them if the zone is the same) in https://skia.googlesource.com/buildbot/+/master/appengine_scripts/skia-tree-status/master_redirect.py"
echo ""
echo
