#!/bin/bash
#
# Setup BugDroid on the specified VM instance.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

function usage() {
  cat << EOF

usage: $0 options

This scripts setups up a specified GCE instance with the ability to run
BugDroid.

OPTIONS:
  -h Show this message
  -i The hostname of the GCE instance we want to setup to run BugDroid.
EOF
}

while getopts "hi:" OPTION
do
  case $OPTION in
    h)
      usage
      exit 1
      ;;
    i)
      VM_HOSTNAME=$OPTARG
      ;;
    ?)
      usage
      exit
      ;;
  esac
done

if [[ -z $VM_HOSTNAME  ]]
then
  usage
  exit 1
fi

source vm_config.sh
source vm_setup_utils.sh

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_BUGDROID[@]}; do
  if [ ! -f $REQUIRED_FILE ];
  then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done

echo """

=================================================
Starting setup of bugdroid on ${VM_HOSTNAME}.....
=================================================

"""

echo "===== Create required directories ====="
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_HOSTNAME \
  "mkdir -p $SKIA_REPO_DIR/bugdroid/repos" \
  || echo "Failed to set up required directories"

echo "===== Checkout buildbot ====="
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_HOSTNAME \
  "cd $SKIA_REPO_DIR/bugdroid && " \
  "/home/$PROJECT_USER/$SKIA_REPO_DIR/depot_tools/gclient config https://skia.googlesource.com/buildbot.git && " \
  "/home/$PROJECT_USER/$SKIA_REPO_DIR/depot_tools/gclient sync" \
  || echo "Failed to checkout buildbot"
echo

echo "===== Checkout repos for bugdroid to watch ====="
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_HOSTNAME \
  "cd $SKIA_REPO_DIR/bugdroid/repos && " \
  "/home/$PROJECT_USER/$SKIA_REPO_DIR/depot_tools/gclient config https://skia.googlesource.com/buildbot.git && " \
  "/home/$PROJECT_USER/$SKIA_REPO_DIR/depot_tools/gclient sync && " \
  "/home/$PROJECT_USER/$SKIA_REPO_DIR/depot_tools/gclient config https://skia.googlesource.com/skia.git && " \
  "/home/$PROJECT_USER/$SKIA_REPO_DIR/depot_tools/gclient sync" \
  || echo "Failed to checkout buildbot"
echo

echo "===== Copying over required master files. ====="
for REQUIRED_FILE in ${REQUIRED_FILES_FOR_BUGDROID[@]}; do
  $GCOMPUTE_CMD push --ssh_user=$PROJECT_USER $VM_HOSTNAME \
    $REQUIRED_FILE $SKIA_REPO_DIR/bugdroid/buildbot/
done
echo

echo "===== Install required packages. ====="
$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_HOSTNAME \
  "sudo pip install gdata" \
  || echo "Failed to install required packages"
echo

echo
echo "To turn on bugdroid for the skia and buildbot repos run the following:"
echo "* cd $SKIA_REPO_DIR/bugdroid/buildbot/services/bugdroid"
echo "* nohup python bugdroid_git.py --repo ~/$SKIA_REPO_DIR/bugdroid/repos/buildbot --log-file-name buildbot-bugdroid-log.txt &"
echo "* nohup python bugdroid_git.py --repo ~/$SKIA_REPO_DIR/bugdroid/repos/skia --log-file-name skia-bugdroid-log.txt &"
echo

