#!/bin/bash
#
# This file contains constants for the shell scripts which interact
# with the skia-webtry-b Google Compute Engine instance.
#
# Copyright 2014 Google Inc. All Rights Reserved.

# The Project ID is found in the Compute tab of the dev console.
# https://code.google.com/apis/console/?pli=1#project:31977622648:overview
PROJECT_ID="google.com:skia-buildbots"

# The user id which owns the server on the vm instance.
PROJECT_USER="default"

# The zone where this instance is on.
ZONE=us-central1-f

# The name of instance where skfiddle.com is running on.
INSTANCE_NAME=skia-webtry-b

# The scope to use for image creation.
SCOPES="https://www.googleapis.com/auth/devstorage.full_control"

WEBTRY_IP_ADDRESS=108.170.219.69
WEBTRY_MACHINE_TYPE=n1-highmem-8
WEBTRY_IMAGE=backports-debian-7-wheezy-v20140331
