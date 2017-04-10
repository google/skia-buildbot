#!/bin/bash
#
# Creates the compute instance for skia-fiddle2.
#
set -x -e

source vm_config.sh

# curl http://us.download.nvidia.com/XFree86/Linux-x86_64/340.65/NVIDIA-Linux-x86_64-340.65.run

gcloud compute copy-files ../common/format_and_mount.sh $PROJECT_USER@$INSTANCE_NAME:/tmp/format_and_mount.sh --zone $ZONE
gcloud compute copy-files ../common/safe_format_and_mount $PROJECT_USER@$INSTANCE_NAME:/tmp/safe_format_and_mount --zone $ZONE

gcloud compute copy-files install.sh $PROJECT_USER@$INSTANCE_NAME:/tmp/install.sh --zone $ZONE
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "/tmp/install.sh" \
  || echo "Installation failure."

