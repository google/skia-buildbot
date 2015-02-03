#!/bin/bash
#
# Utility functions for the Skia GCE setup scripts.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)


function install_packages {
  # Add new packages that are not yet part of the image below.
  echo
  echo "Install Required packages"
  # $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
  #   "sudo apt-get -y install " \
  #   || FAILED="$FAILED InstallPackages"
  echo
}

function setup_symlinks {
  # Add new symlinks that are not yet part of the image below.
  echo
  echo "Setup Symlinks"
  # $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
  #   "sudo ln -s -f /usr/bin/ccache /usr/local/bin/cc" \
  #   || FAILED="$FAILED InstallPackages"
  echo
}

function setup_android_sdk {
  echo
  echo "===== Android SDK. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "mkdir $SKIA_REPO_DIR;" \
    "cd $SKIA_REPO_DIR && " \
    "sudo apt-get -y install libncurses5:i386 && " \
    "wget http://dl.google.com/android/adt/adt-bundle-linux-x86_64-20130729.zip && " \
    "if [[ -d adt-bundle-linux-x86_64-20130729 ]]; then rm -rf adt-bundle-linux-x86_64-20130729; fi && " \
    "unzip adt-bundle-linux-x86_64-20130729.zip && " \
    "if [[ -d android-sdk-linux ]]; then rm -rf android-sdk-linux; fi && " \
    "mv adt-bundle-linux-x86_64-20130729/sdk android-sdk-linux && " \
    "rm -rf adt-bundle-linux-x86_64-20130729 adt-bundle-linux-x86_64-20130729.zip && " \
    "echo \"y\" | android-sdk-linux/tools/android update sdk --no-ui --filter android-19 && " \
    "echo 'export ANDROID_SDK_ROOT=$SKIA_REPO_DIR/android-sdk-linux' >> /home/$PROJECT_USER/.bashrc" \
    || FAILED="$FAILED AndroidSDK"
  echo
}

function setup_nacl {
  NACL_PEPPER_VERSION="pepper_32"
  echo
  echo "===== Native Client SDK and NaClPorts. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "mkdir $SKIA_REPO_DIR;" \
    "cd $SKIA_REPO_DIR && " \
    "wget http://storage.googleapis.com/nativeclient-mirror/nacl/nacl_sdk/nacl_sdk.zip && " \
    "if [[ -d nacl_sdk ]]; then rm -rf nacl_sdk; fi && " \
    "unzip nacl_sdk.zip && " \
    "rm nacl_sdk.zip && " \
    "nacl_sdk/naclsdk update $NACL_PEPPER_VERSION && " \
    "echo 'export NACL_SDK_ROOT=$SKIA_REPO_DIR/nacl_sdk/$NACL_PEPPER_VERSION' >> /home/$PROJECT_USER/.bashrc" \
    || FAILED="$FAILED NativeClient"
  echo
}

function checkout_skia_repos {
  echo
  echo "Checkout Skia Buildbot code"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "mkdir $SKIA_REPO_DIR;" \
    "cd $SKIA_REPO_DIR && " \
    "~/depot_tools/gclient config https://skia.googlesource.com/buildbot.git && " \
    "~/depot_tools/gclient sync;" \
    || FAILED="$FAILED CheckoutSkiaBuildbot"
  echo
}

function copy_files {
  echo
  echo "===== Copying over required files. ====="
    for REQUIRED_FILE in ${REQUIRED_FILES_FOR_BOTS[@]}; do
      $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $INSTANCE_NAME \
        $REQUIRED_FILE /home/$PROJECT_USER/
      $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $INSTANCE_NAME \
        $REQUIRED_FILE /home/$PROJECT_USER/storage/
      $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $INSTANCE_NAME \
        $REQUIRED_FILE /home/$PROJECT_USER/storage/skia-repo/
    done
  echo
}

function reboot {
  echo
  echo "===== Rebooting the instance ======"
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $INSTANCE_NAME \
    "sudo reboot" \
    || FAILED="$FAILED Reboot"
  echo
}
