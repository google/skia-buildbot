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
VM_MASTER_NAMES=${VM_MASTER_NAMES:="master private-master fyi-master android-master compile-master experimental-master"}
MASTER_MACHINE_TYPE="n1-highmem-2"
MASTER_IP_ADDRESSES_a="108.170.220.117 108.170.220.115 108.170.220.123 108.170.220.122 108.170.220.75 108.170.220.127"
MASTER_IP_ADDRESSES_b="108.170.220.120 108.170.220.27 108.170.220.102 108.170.220.21 108.170.220.76 108.170.220.89"

# Slave names, type and their IP addresses.
VM_SLAVE_NAMES=${VM_SLAVE_NAMES:="housekeeping-slave compile1 compile2 compile3 compile4 compile5"}
SLAVES_MACHINE_TYPE="n1-standard-2"
# The list of slave names must correspond to the list of their IP addresses.
# If you need to add more slaves find available IP addresses by running:
#   gcutil --project=google.com:skia-buildbots listaddresses
SLAVE_IP_ADDRESSES="108.170.220.73 108.170.220.26 108.170.220.96 108.170.220.108 108.170.220.55 108.170.220.69"
# The following IP addresses are reserved for skia-android-canary:
# 108.170.220.94
# The following IP address is used by skia-webtry (jcgregorio's instance):
# 108.170.220.126
# The following IP address is used by skia-perf (bensong's instance):
# 108.170.220.208
# The following IP address is used by skia-monitor (jcgregorio's instance):
MONITORING_IP_ADDRESS=108.170.220.59
MONITORING_IMAGE=backports-debian-7-wheezy-v20140331
MONITORING_MACHINE_TYPE=n1-standard-1

# rebaseline_server names, type and their IP addresses to use for each zone.
VM_REBASELINESERVER_NAMES=${VM_REBASELINESERVER_NAMES:="rebaseline-server-1"}
REBASELINESERVER_MACHINE_TYPE="g1-small"
REBASELINESERVER_IP_ADDRESSES_a="108.170.220.121"
REBASELINESERVER_IP_ADDRESSES_b=$REBASELINESERVER_IP_ADDRESSES_a

# The Skia buildbot GCE image name.
SKIA_BUILDBOT_IMAGE_NAME_V1="skia-buildbot-image-v1"

# The scope to use for image creation.
SCOPES="https://www.googleapis.com/auth/devstorage.full_control"

# Define required files for various instance types.
REQUIRED_FILES_FOR_MASTER=(~/.code_review_password \
                           ~/.status_password \
                           ~/.skia_buildbots_password)

# TODO(epoger): Once a master restart has picked up
# https://codereview.chromium.org/320893002/ , we can delete the autogen lines.
REQUIRED_FILES_FOR_SLAVES=(~/.autogen_svn_username \
                           ~/.autogen_svn_password \
                           ~/.skia_svn_username \
                           ~/.skia_svn_password \
                           ~/.boto)

REQUIRED_FILES_FOR_REBASELINESERVER=(files-to-copy/kick-rebaseline-server.sh \
                                     files-to-copy/rebaseline-server-crontab)

# The directory where the scratch disk is mounted.
SKIA_REPO_DIR="skia-repo"

# The (Shared Fate) Zone is conceptually equivalent to a data center cell. VM
# instances live in a zone.
#
# We flip the default one as required by PCRs in bigcluster. We are allowed
# us-central1-a and us-central1-b.
# A short tag to use as part of the VM instance name
ZONE_TAG=${ZONE_TAG:=b}
# Make sure ZONE_TAG is either 'b' or 'a', they are the only ones allowed.
if [ "$ZONE_TAG" == "a" ]; then
  OLD_ZONE_TAG="b"
elif [ "$ZONE_TAG" == "b" ]; then
  OLD_ZONE_TAG="a"
else
  echo "ZONE_TAG=$ZONE_TAG has to be one of \"a\" or \"b\"."
  exit 1
fi

ZONE=us-central2-$ZONE_TAG

# The Project ID is found in the Compute tab of the dev console.
# https://code.google.com/apis/console/?pli=1#project:31977622648:overview
PROJECT_ID="google.com:skia-buildbots"

# The user id which owns the server on the vm instance
PROJECT_USER="default"

# gcutil commands.
GCUTIL=`which gcutil`
GCOMPUTE_CMD="$GCUTIL --cluster=prod --project=$PROJECT_ID"
GCOMPUTE_SSH_CMD="$GCOMPUTE_CMD ssh --ssh_user=$PROJECT_USER"
