#!/bin/bash
#
# Setup all the master buildbot instances.
#
# Copyright 2012 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

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

  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "sudo apt-get update; " \
    "sudo apt-get install git make subversion postfix python-dev; " \
    "echo 'PATH=\"~/skia-master/depot_tools:/usr/local/sbin:/usr/sbin:/sbin:$PATH\"' >> ~/.bashrc; " \
    "echo 'alias m=\"cd ~/skia-master/buildbot/master\"' >> ~/.bashrc; " \
    "sudo easy_install --upgrade google-api-python-client; " \
    "sudo easy_install --upgrade pyOpenSSL; " \
    "sudo sh -c 'echo 127.0.0.1 smtp >> /etc/hosts'"

   echo """
Please manually ssh into ${VM_COMPLETE_NAME} from a different terminal and follow all the steps in:
  http://www.zoneminder.com/wiki/index.php/How_to_install_and_configure_Postfix_as_a_Gmail_SMTP_relay_for_ZoneMinder_email_filter_events.

ssh cmd: ${GCOMPUTE_CMD} ssh --ssh_user=default ${VM_COMPLETE_NAME}
"""

  unset USER_INPUT
  echo "Please enter 'y' when you are ready to proceed."
  while [ "$USER_INPUT" != "y" ]; do
    read -n 1 USER_INPUT
  done

  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "mkdir ~/skia-master; " \
    "sudo /usr/share/google/safe_format_and_mount /dev/${VM}-disk-${ZONE_TAG} /home/default/skia-master; " \
    "cd ~/skia-master; " \
    "sudo chmod 777 -R .; " \
    "svn checkout http://src.chromium.org/svn/trunk/tools/depot_tools; " \
    "~/skia-master/depot_tools/gclient config https://skia.googlecode.com/svn/buildbot; " \
    "~/skia-master/depot_tools/gclient sync; "

  for REQUIRED_FILE in ${REQUIRED_FILES_FOR_MASTERS[@]}; do
    $GCOMPUTE_CMD push --ssh_user=default $VM_COMPLETE_NAME \
      $REQUIRED_FILE /home/default/skia-master/buildbot/master/
  done

done

