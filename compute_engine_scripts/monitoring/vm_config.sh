#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the monitoring Google Compute Engine instance.
#
# Copyright 2014 Google Inc. All Rights Reserved.

# The base names of the VM instances. Actual names are VM_NAME_BASE-name-zone
VM_NAME_BASE=${VM_NAME_BASE:="skia"}

MONITORING_IP_ADDRESS=108.170.219.115
MONITORING_IMAGE=backports-debian-7-wheezy-v20140415
MONITORING_MACHINE_TYPE=n1-standard-1

# The scope to use for image creation.
SCOPES="https://www.googleapis.com/auth/devstorage.full_control"

# The (Shared Fate) Zone is conceptually equivalent to a data center cell. VM
# instances live in a zone.
#
# A short tag to use as part of the VM instance name
ZONE_TAG=${ZONE_TAG:=f}

ZONE=us-central1-$ZONE_TAG

# The Project ID is found in the Compute tab of the dev console.
# https://code.google.com/apis/console/?pli=1#project:31977622648:overview
PROJECT_ID="google.com:skia-buildbots"

# The user id which owns the server on the vm instance
PROJECT_USER="default"

