#!/bin/bash
#
# Setup all the slave buildbot instances.
#
# Copyright 2012 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_SLAVES[@]}; do
  if [ ! -f $REQUIRED_FILE ];
  then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done

for VM in $VM_SLAVE_NAMES; do
  VM_COMPLETE_NAME="${VM_NAME_BASE}-${VM}-${ZONE_TAG}"

  echo """

================================================
Starting setup of ${VM_COMPLETE_NAME}.....
================================================

"""

  $GCOMPUTE_CMD ssh --ssh_user=default $VM_COMPLETE_NAME \
    "sudo apt-get update; " \
    "sudo apt-get install libfreetype6 libfreetype6-dev libpng12-0 libpng12-dev libgl1-mesa-dev " \
    "libgl1-mesa-dri subversion postfix libgl1-mesa-glx libglu1-mesa libglu1-mesa-dev mesa-common-dev " \
    "libosmesa6 libosmesa6-dev doxygen clang libQt4-dev git make python-dev g++ python-epydoc; " \
    "echo 'PATH=\"~/skia-slave/depot_tools:/usr/local/sbin:/usr/sbin:/sbin:$PATH\"' >> ~/.bashrc; " \
    "echo 'alias s=\"cd ~/skia-slave/buildbot/slave\"' >> ~/.bashrc; " \
    "sudo easy_install --upgrade google-api-python-client; " \
    "sudo easy_install --upgrade pyOpenSSL; " \
    "mkdir ~/skia-slave; " \
    "sudo /usr/share/google/safe_format_and_mount /dev/${VM}-disk-${ZONE_TAG} /home/default/skia-slave; " \
    "cd ~/skia-slave; " \
    "sudo chmod 777 -R .; " \
    "svn checkout http://src.chromium.org/svn/trunk/tools/depot_tools; " \
    "~/skia-slave/depot_tools/gclient config https://skia.googlecode.com/svn/buildbot; " \
    "~/skia-slave/depot_tools/gclient sync; "

  for REQUIRED_FILE in ${REQUIRED_FILES_FOR_SLAVES[@]}; do
    $GCOMPUTE_CMD push --ssh_user=default $VM_COMPLETE_NAME \
      $REQUIRED_FILE /home/default/skia-slave/buildbot/site_config/
    $GCOMPUTE_CMD push --ssh_user=default $VM_COMPLETE_NAME \
      $REQUIRED_FILE /home/default/skia-slave/buildbot/third_party/chromium_buildbot/site_config/
  done

  echo """
Please manually ssh into ${VM_COMPLETE_NAME} and:
  * Generate SSH keys (ssh-keygen -t dsa).
  * Add the public key to the slave's ~/.ssh/authorized_keys
  * Add the public key to the buildbot master's ~/.ssh/authorized_keys

ssh cmd: ${GCOMPUTE_CMD} ssh --ssh_user=default ${VM_COMPLETE_NAME}
"""

done

