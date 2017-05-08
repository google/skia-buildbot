#!/bin/bash
#
# Creates the compute instance for skia-fiddle.
#
set -x -e

source vm_config.sh

FIDDLE_MACHINE_TYPE=n1-standard-8
FIDDLE_SOURCE_SNAPSHOT=skia-systemd-pushable-base
FIDDLE_SCOPES='https://www.googleapis.com/auth/devstorage.full_control'

gcloud compute copy-files ../common/format_and_mount.sh $PROJECT_USER@$INSTANCE_NAME:/tmp/format_and_mount.sh --zone $ZONE
gcloud compute copy-files ../common/safe_format_and_mount $PROJECT_USER@$INSTANCE_NAME:/tmp/safe_format_and_mount --zone $ZONE

gcloud compute copy-files install.sh $PROJECT_USER@$INSTANCE_NAME:/tmp/install.sh --zone $ZONE
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "/tmp/install.sh" \
  || echo "Installation failure."

