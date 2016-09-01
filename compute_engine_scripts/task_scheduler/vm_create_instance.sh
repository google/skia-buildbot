#!/bin/bash
#
# Creates the compute instance for skia-task-scheduler
#
set -x

source vm_config.sh

REQUIRED_FILES=(/tmp/.gitconfig \
                /tmp/.netrc)

# Check that all required files exist.
for REQUIRED_FILE in ${REQUIRED_FILES[@]}; do
  if [ ! -f $REQUIRED_FILE ]; then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done

MACHINE_TYPE=n1-highmem-16
SOURCE_SNAPSHOT=skia-systemd-pushable-base
SCOPES='https://www.googleapis.com/auth/devstorage.full_control https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile'
IP_ADDRESS=104.154.112.128

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $SOURCE_SNAPSHOT \
  --type "pd-standard"

# Create a large data disk.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME"-data" \
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
  --tags "http-server" "https-server" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --metadata "owner_primary=borenet,owner_secondary=benjaminwagner" \
  --disk name=${INSTANCE_NAME}      device-name=${INSTANCE_NAME}      "mode=rw" "boot=yes" "auto-delete=yes" \
  --disk name=${INSTANCE_NAME}-data device-name=${INSTANCE_NAME}-data "mode=rw" "boot=no" \
  --address=$IP_ADDRESS

# Wait until the instance is up.
until nc -w 1 -z $IP_ADDRESS 22; do
    echo "Waiting for VM to come up."
    sleep 2
done

gcloud compute copy-files ../common/format_and_mount.sh $PROJECT_USER@$INSTANCE_NAME:/tmp/format_and_mount.sh --zone $ZONE
gcloud compute copy-files ../common/safe_format_and_mount $PROJECT_USER@$INSTANCE_NAME:/tmp/safe_format_and_mount --zone $ZONE

gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "/tmp/format_and_mount.sh $INSTANCE_NAME" \
  || echo "Installation failure."

echo
echo "===== Copying over required files. ====="
  for REQUIRED_FILE in ${REQUIRED_FILES[@]}; do
    echo "Copy ${REQUIRED_FILE}"
    gcloud compute --project $PROJECT_ID copy-files $REQUIRED_FILE $PROJECT_USER@$INSTANCE_NAME:/home/$PROJECT_USER/ --zone $ZONE
  done
echo

# The instance believes it is skia-systemd-snapshot-maker until it is rebooted.
echo
echo "===== Rebooting the instance ======"
# Using "shutdown -r +1" rather than "reboot" so that the connection isn't
# terminated immediately, which causes a non-zero exit code.
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "sudo shutdown -r +1" \
  || echo "Reboot failed; please reboot the instance manually."
