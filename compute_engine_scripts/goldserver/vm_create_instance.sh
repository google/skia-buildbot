#!/bin/bash
#
# Creates the compute instance for skiagold.
#
set -x

source vm_config.sh

# # Create a large data disk.
gcloud compute --project $PROJECT_ID disks create $GOLD_DATA_DISK_NAME \
  --size $GOLD_DATA_DISK_SIZE \
  --zone $ZONE \
  --type "pd-standard"


## TODO(stephan): Switch back to using gcloud instead of gcutil.
#  The command using gcloud is preferrable, but adding scopes doesn't
#  currently work, that's why we reverted to using gcutil. The
#  gcloud command has to be adapted to use an image instead of a snapshot.
#
# gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
#   --zone $ZONE \
#   --machine-type $GOLD_MACHINE_TYPE \
#   --network "default" \
#   --maintenance-policy "MIGRATE" \
#   --scopes "$GOLD_SCOPES" \
#   --tags "http-server" "https-server" \
#   --disk name=${INSTANCE_NAME}      device-name=${INSTANCE_NAME}      "mode=rw" "boot=yes" "auto-delete=yes" \
#   --disk name=${GOLD_DATA_DISK_NAME} device-name=${GOLD_DATA_DISK_NAME} "mode=rw" "boot=no" \
#   --metadata-from-file "startup-script=startup-script.sh" \
#   --address $GOLD_IP_ADDRESS \
#   --verbosity debug

# Create the instance based on the
gcutil --project=$PROJECT_ID addinstance ${INSTANCE_NAME} \
  --zone=$ZONE \
  --machine_type=$GOLD_MACHINE_TYPE \
  --network="default" \
  --on_host_maintenance="migrate" \
  --service_account=$PROJECT_USER \
  --service_account_scopes="$GOLD_SCOPES" \
  --tags="http-server,https-server" \
  --image="${GOLD_SOURCE_IMAGE}" \
  --disk="${GOLD_DATA_DISK_NAME},deviceName=${GOLD_DATA_DISK_NAME},mode=rw" \
  --metadata_from_file=startup-script:startup-script.sh \
  --external_ip_address=$GOLD_IP_ADDRESS \
  --boot_disk_size_gb=10 \
  --auto_delete_boot_disk \
  --wait_until_running \
  --log_level=DEBUG
