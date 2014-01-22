#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the Google Compute Engine instances.
#
# Copyright 2012 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

# The base names of the VM instances. Actual names are VM_NAME_BASE-name-zone
VM_NAME_BASE=${VM_NAME_BASE:="skia"}

# Master names, type and their IP addresses to use for each zone.
VM_MASTER_NAMES=${VM_MASTER_NAMES:="master private-master fyi-master"}
MASTER_MACHINE_TYPE="n1-highmem-2"
MASTER_IP_ADDRESSES_a="108.170.217.252 8.34.217.98 108.170.219.160"
MASTER_IP_ADDRESSES_b="173.255.115.253 8.34.217.86 108.170.219.161"

# Slave names, type and their IP addresses.
VM_SLAVE_NAMES=${VM_SLAVE_NAMES:="housekeeping-slave compile1 compile2 compile3 compile4 compile5"}
SLAVES_MACHINE_TYPE="n1-standard-2-d"
# The list of slave names must correspond to the list of their IP addresses.
# If you need to add more slaves find available IP addresses by running:
#   gcutil --project=google.com:skia-buildbots listaddresses
SLAVE_IP_ADDRESSES_a="173.255.114.84 173.255.114.239 173.255.114.128 108.170.217.249 173.255.115.61 108.170.217.254"
SLAVE_IP_ADDRESSES_b="108.170.217.253 108.170.217.250 108.170.217.251 108.170.217.247 108.170.217.248 108.170.217.246"
# The following IP addresses are reserved for skia-android-canary:
# 108.170.219.169 amd 108.170.219.168

# The Skia buildbot GCE image name.
SKIA_BUILDBOT_IMAGE_NAME="skia-buildbot-image"

# The scope to use for image creation.
SCOPES="https://www.googleapis.com/auth/devstorage.full_control"

# Define required files for master and slaves.
REQUIRED_FILES_FOR_MASTER=(~/.code_review_password \
                           ~/.status_password \
                           ~/.skia_buildbots_password)

REQUIRED_FILES_FOR_SLAVES=(~/.autogen_svn_username \
                           ~/.autogen_svn_password \
                           ~/.skia_svn_username \
                           ~/.skia_svn_password \
                           ~/.boto)

# The directory where the scratch disk is mounted.
SKIA_REPO_DIR="skia-repo"

# The (Shared Fate) Zone is conceptually equivalent to a data center cell. VM
# instances live in a zone.
#
# We flip the default one as required by PCRs in bigcluster. We are allowed
# us-central1-a and us-central1-b.
# A short tag to use as part of the VM instance name
ZONE_TAG=${ZONE_TAG:=a}
# Make sure ZONE_TAG is either 'b' or 'a', they are the only ones allowed.
if [ "$ZONE_TAG" == "a" ]; then
  OLD_ZONE_TAG="b"
elif [ "$ZONE_TAG" == "b" ]; then
  OLD_ZONE_TAG="a"
else
  echo "ZONE_TAG=$ZONE_TAG has to be one of \"a\" or \"b\"."
  exit 1
fi

ZONE=us-central1-$ZONE_TAG

# The Project ID is found in the Compute tab of the dev console.
# https://code.google.com/apis/console/?pli=1#project:31977622648:overview
PROJECT_ID="google.com:skia-buildbots"

# The user id which owns the server on the vm instance
PROJECT_USER="default"

# gcutil commands.
GCUTIL=`which gcutil`
GCOMPUTE_CMD="$GCUTIL --cluster=prod --project=$PROJECT_ID"
GCOMPUTE_SSH_CMD="$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER"
