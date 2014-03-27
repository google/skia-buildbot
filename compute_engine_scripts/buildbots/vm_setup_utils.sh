#!/bin/bash
#
# Utility functions for the GCE buildbot scripts.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

function checkout_depot_tools {
  echo
  echo "===== Checkout depot tools. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
    "mkdir -p $SKIA_REPO_DIR; " \
    "cd $SKIA_REPO_DIR && " \
    "svn checkout http://src.chromium.org/svn/trunk/tools/depot_tools" \
    || FAILED="$FAILED DepotTools"
  echo
}

function checkout_buildbot {
  echo
  echo "===== Checkout the buildbot code. ====="
  $GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER $VM_COMPLETE_NAME \
    "mkdir -p $SKIA_REPO_DIR; " \
    "cd $SKIA_REPO_DIR && " \
    "/home/$PROJECT_USER/$SKIA_REPO_DIR/depot_tools/gclient config https://skia.googlesource.com/buildbot.git && " \
    "/home/$PROJECT_USER/$SKIA_REPO_DIR/depot_tools/gclient sync" \
    || FAILED="$FAILED BuildbotScripts"
  echo
}
