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

VM_COMPLETE_NAME="${VM_NAME_BASE}-${VM_MASTER_NAME}-${ZONE_TAG}"

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
