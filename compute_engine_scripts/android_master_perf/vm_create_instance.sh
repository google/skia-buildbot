#!/bin/bash
#
# Creates the compute instance for skia-android-master-perf
#
set -x

source vm_config.sh

MACHINE_TYPE=n1-standard-32
SOURCE_SNAPSHOT=skia-systemd-pushable-base
SCOPES='https://www.googleapis.com/auth/devstorage.full_control'
DISK_NAME="$INSTANCE_NAME-ssd-data"
IP_ADDRESS=104.154.112.139

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $SOURCE_SNAPSHOT \
  --type "pd-standard"

# Create a large data disk.
gcloud compute --project $PROJECT_ID disks create $DISK_NAME \
  --size "1000" \
  --zone $ZONE \
  --type "pd-ssd"

# Create the instance with the two disks attached.
gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $SCOPES \
  --tags "http-server,https-server" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --metadata "owner_primary=jcgregorio,owner_secondary=stephana" \
  --disk name=${INSTANCE_NAME},device-name=${INSTANCE_NAME},mode=rw,boot=yes,auto-delete=yes \
  --disk name=${DISK_NAME},device-name=${DISK_NAME},mode=rw,boot=no \
  --address=$IP_ADDRESS

# Wait until the instance is up.
while true; do
  sleep 10
  # Pull out the status of the instance.
  STATUS=`gcloud compute --project $PROJECT_ID instances describe $INSTANCE_NAME --zone $ZONE | grep "^status:" | sed s/status://g`
  if [ $STATUS="RUNNING" ]; then
    break
  fi
done

gcloud compute copy-files ../common/format_and_mount.sh $PROJECT_USER@$INSTANCE_NAME:/tmp/format_and_mount.sh --zone $ZONE
gcloud compute copy-files ../common/safe_format_and_mount $PROJECT_USER@$INSTANCE_NAME:/tmp/safe_format_and_mount --zone $ZONE

gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "/tmp/format_and_mount.sh $INSTANCE_NAME-ssd" \
  || echo "Installation failure."

## The instance believes it is skia-systemd-snapshot-maker until it is rebooted.
#echo
echo "===== Rebooting the instance ======"
# Using "shutdown -r +1" rather than "reboot" so that the connection isn't
# terminated immediately, which causes a non-zero exit code.
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "sudo shutdown -r +1" \
  || echo "Reboot failed; please reboot the instance manually."

