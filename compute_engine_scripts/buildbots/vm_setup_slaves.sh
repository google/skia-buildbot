#!/bin/bash
#
# Setup all the slave buildbot instances.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh
source vm_setup_utils.sh

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

  FAILED=""

  checkout_depot_tools

  checkout_buildbot

  echo
  echo "===== Android SDK. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
    "cd $SKIA_REPO_DIR && " \
    "sudo apt-get -y install libncurses5:i386 && " \
    "wget http://dl.google.com/android/adt/adt-bundle-linux-x86_64-20130729.zip && " \
    "if [[ -d adt-bundle-linux-x86_64-20130729 ]]; then rm -rf adt-bundle-linux-x86_64-20130729; fi && " \
    "unzip adt-bundle-linux-x86_64-20130729.zip && " \
    "if [[ -d android-sdk-linux ]]; then rm -rf android-sdk-linux; fi && " \
    "mv adt-bundle-linux-x86_64-20130729/sdk android-sdk-linux && " \
    "rm -rf adt-bundle-linux-x86_64-20130729 adt-bundle-linux-x86_64-20130729.zip && " \
    "android-sdk-linux/tools/android update sdk --no-ui --filter android-19" \
    || FAILED="$FAILED AndroidSDK"
  echo

  NACL_PEPPER_VERSION="pepper_32"

  echo
  echo "===== Native Client SDK and NaClPorts. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
    "cd $SKIA_REPO_DIR && " \
    "wget http://storage.googleapis.com/nativeclient-mirror/nacl/nacl_sdk/nacl_sdk.zip && " \
    "if [[ -d nacl_sdk ]]; then rm -rf nacl_sdk; fi && " \
    "unzip nacl_sdk.zip && " \
    "rm nacl_sdk.zip && " \
    "nacl_sdk/naclsdk update $NACL_PEPPER_VERSION && " \
    "echo 'export NACL_SDK_ROOT=/home/$PROJECT_USER/$SKIA_REPO_DIR/nacl_sdk/$NACL_PEPPER_VERSION' >> /home/$PROJECT_USER/.bashrc" \
    || FAILED="$FAILED NativeClient"
  echo

  echo
  echo "===== Install missing packages. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
    "sudo apt-get install python-django && " \
    "sudo easy_install zope.interface" \
    || FAILED="$FAILED InstallPackages"
  echo

  echo
  echo "===== Copying over required master and slave files. ====="
  for REQUIRED_FILE in ${REQUIRED_FILES_FOR_SLAVES[@]}; do
    $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \             
      $REQUIRED_FILE /home/$PROJECT_USER/
    $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
      $REQUIRED_FILE /home/$PROJECT_USER/$SKIA_REPO_DIR/
  done

  echo
  echo "===== Setting up launch-on-reboot ======"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
    "cp $SKIA_REPO_DIR/buildbot/scripts/skiabot-vm-slave-start-on-boot.sh . && " \
    "chmod a+x skiabot-vm-slave-start-on-boot.sh && " \
    "echo \"@reboot /home/${PROJECT_USER}/skiabot-vm-slave-start-on-boot.sh ${SKIA_REPO_DIR}\" > reboot.txt && " \
    "crontab -u $PROJECT_USER reboot.txt && " \
    "rm reboot.txt" \
    || FAILED="$FAILED LaunchOnReboot"
  echo

  if [[ $FAILED ]]; then
    echo
    echo "FAILURES: $FAILED"
    echo "Please manually fix these errors."
    echo
  fi

done
