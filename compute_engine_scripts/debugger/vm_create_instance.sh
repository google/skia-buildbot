#!/bin/bash
#
# Creates the compute instance for skia-tracedb.
#
set -x

source vm_config.sh

DEBUGGER_MACHINE_TYPE=n1-standard-1
DEBUGGER_SOURCE_SNAPSHOT=skia-systemd-pushable-base
DEBUGGER_SCOPES='https://www.googleapis.com/auth/devstorage.full_control'
DEBUGGER_IP_ADDRESS=104.154.112.116

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $DEBUGGER_SOURCE_SNAPSHOT \
  --type "pd-standard"

# Create the instance with the two disks attached.
gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $DEBUGGER_MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $DEBUGGER_SCOPES \
  --tags "http-server" "https-server" \
  --metadata "owner_primary=jcgregorio" \
  --disk name=${INSTANCE_NAME}      device-name=${INSTANCE_NAME}      "mode=rw" "boot=yes" "auto-delete=yes" \
  --address=$DEBUGGER_IP_ADDRESS

# The instance believes it is skia-systemd-snapshot-maker until it is rebooted.
echo
echo "===== Rebooting the instance ======"
# Using "shutdown -r +1" rather than "reboot" so that the connection isn't
# terminated immediately, which causes a non-zero exit code.
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "sudo shutdown -r +1" \
  || echo "Reboot failed; please reboot the instance manually."
