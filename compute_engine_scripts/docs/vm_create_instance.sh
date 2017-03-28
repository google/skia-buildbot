#!/bin/bash
#
# Creates the compute instance for skiadocs.
#
set -x

source vm_config.sh

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot skia-systemd-pushable-base \
  --type "pd-standard"

set +e
# The cmd may fail if the disk already exists, which is fine.
# Create a large data disk.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME"-data" \
  --size "1000" \
  --zone $ZONE \
  --type "pd-standard"
set -e

gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $MACHINE_TYPE \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $SCOPES \
  --tags http-server,https-server \
  --metadata "owner_primary=jcgregorio" \
  --disk name=$INSTANCE_NAME,device-name=${INSTANCE_NAME},mode=rw,boot=yes,auto-delete=yes \
  --disk name=${INSTANCE_NAME}-data,device-name=${INSTANCE_NAME}-data,mode=rw,boot=no \
  --metadata-from-file "startup-script=startup-script.sh" \
  --address $DOCS_IP_ADDRESS

# The instance believes it is skia-systemd-snapshot-maker until it is rebooted.
echo
echo "===== Rebooting the instance ======"
# Using "shutdown -r +1" rather than "reboot" so that the connection isn't
# terminated immediately, which causes a non-zero exit code.
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "sudo shutdown -r +1" \
  || echo "Reboot failed; please reboot the instance manually."

# Wait for shutdown.
sleep 120

# Wait until the instance is up.
until nc -w 1 -z $DOCS_IP_ADDRESS 22; do
    echo "Waiting for VM to come up."
    sleep 2
done

gcloud compute copy-files ../common/format_and_mount.sh $PROJECT_USER@$INSTANCE_NAME:/tmp/format_and_mount.sh --zone $ZONE
gcloud compute copy-files ../common/safe_format_and_mount $PROJECT_USER@$INSTANCE_NAME:/tmp/safe_format_and_mount --zone $ZONE
gcloud compute copy-files install.sh $PROJECT_USER@$INSTANCE_NAME:/tmp/install.sh --zone $ZONE
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "/tmp/format_and_mount.sh $INSTANCE_NAME" \
  || echo "Installation failure."

