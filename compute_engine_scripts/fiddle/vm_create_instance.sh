#!/bin/bash
#
# Creates the compute instance for skia-fiddle.
#
set -x -e

source vm_config.sh

FIDDLE_MACHINE_TYPE=n1-standard-4
FIDDLE_SOURCE_SNAPSHOT=skia-systemd-pushable-base
FIDDLE_SCOPES='https://www.googleapis.com/auth/devstorage.full_control'
FIDDLE_IP_ADDRESS=104.154.112.126

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $FIDDLE_SOURCE_SNAPSHOT \
  --type "pd-standard"

set +e
# The cmd may fail if the disk already exists, which is fine.
# Create a large data disk.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME"-data" \
  --size "1000" \
  --zone $ZONE \
  --type "pd-standard"
set -e

# Create the instance with the two disks attached.
gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $FIDDLE_MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $FIDDLE_SCOPES \
  --tags "http-server,https-server" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --disk "name=${INSTANCE_NAME},device-name=${INSTANCE_NAME},mode=rw,boot=yes,auto-delete=yes" \
  --disk "name=${INSTANCE_NAME}-data,device-name=${INSTANCE_NAME}-data,mode=rw,boot=no" \
  --address=$FIDDLE_IP_ADDRESS

# Wait until the instance is up.
until nc -w 1 -z $FIDDLE_IP_ADDRESS 22; do
    echo "Waiting for VM to come up."
    sleep 2
done

gcloud compute copy-files install.sh $PROJECT_USER@$INSTANCE_NAME:/tmp/install.sh --zone $ZONE
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "/tmp/install.sh" \
  || echo "Installation failure."

# The instance believes it is skia-systemd-snapshot-maker until it is rebooted.
echo
echo "===== Rebooting the instance ======"
# Using "shutdown -r +1" rather than "reboot" so that the connection isn't
# terminated immediately, which causes a non-zero exit code.
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "sudo shutdown -r +1" \
  || echo "Reboot failed; please reboot the instance manually."
