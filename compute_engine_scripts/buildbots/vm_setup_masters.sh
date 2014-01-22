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
echo "After the new master instance is turned on, CQ and Rietveld needs to be"
echo "updated. The steps to do this are:"
echo "* Create a CQ CL updating the IP Address. Eg: "
echo "  https://codereview.chromium.org/22859026/"
echo "* Make a list of all Skia base URLs in Rietveld by running (will need"
echo "  access to chromiumcodereview-hr):"
echo "  https://appengine.google.com/datastore/explorer?submitted=1&app_id=s~chromiumcodereview-hr&show_options=yes&version_id=1129-927a0ebf0e29.369505670897398781&viewby=gql&query=SELECT+*+FROM+BaseUrlTryServer+WHERE+tryserver_name%3D%27tryserver.skia%27&options=Run+Query"
echo "* Enter the new json_url in"
echo "  https://codereview.chromium.org/restricted/update_tryservers."
echo "* Flip the order of the MASTER_IPS in https://skia.googlesource.com/buildbot/+/master/appengine_scripts/skia-tree-status/master_redirect.py"
echo ""
echo
