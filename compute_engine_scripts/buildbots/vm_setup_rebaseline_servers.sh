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

  # TODO(rmistry): The skia-buildbot-image-v1 image has an incorrect symlink
  # from /tmp to ~/skia-repo. This happened because /tmp was not big enough to
  # capture the image using GCE scripts. Once the symlink was created the image
  # could be captured but the symlink ended up persisting in the image.
  echo
  echo "===== Fix symlink from /tmp to ~/skia-repo ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
    "sudo unlink /tmp && " \
    "sudo mkdir /tmp && " \
    "sudo chmod 1777 /tmp" \
    || echo "Failed to fix /tmp!"
  echo

  echo
  echo "===== Reboot instance to automatically start the server ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
    "sudo reboot" \
    || echo "Failed to reboot the instance!"
  echo

done
