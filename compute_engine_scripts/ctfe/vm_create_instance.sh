#!/bin/bash
#
# Creates the compute instance for skia-ctfe.
#
set -x

source vm_config.sh

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $CTFE_SOURCE_SNAPSHOT \
  --type "pd-standard"

gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $CTFE_MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $CTFE_SCOPES \
  --tags "http-server" "https-server" \
  --disk "name=skia-ctfe" "device-name=skia-ctfe" "mode=rw" "boot=yes" "auto-delete=yes" \
  --address $CTFE_IP_ADDRESS

WAIT_TIME_AFTER_CREATION_SECS=600
echo
echo "===== Wait $WAIT_TIME_AFTER_CREATION_SECS secs for instance to" \
  "complete startup script. ====="
echo
sleep $WAIT_TIME_AFTER_CREATION_SECS

# The instance believes it is skia-systemd-snapshot-maker until it is rebooted.
echo
echo "===== Rebooting the instance ======"
# Using "shutdown -r +1" rather than "reboot" so that the connection isn't
# terminated immediately, which causes a non-zero exit code.
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "sudo shutdown -r +1" \
  || echo "Reboot failed; please reboot the instance manually."
