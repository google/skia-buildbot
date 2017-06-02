#!/bin/bash
#
# Creates the compute instance for skia-fiddle.
#
set -x -e

source vm_config.sh

FIDDLE_MACHINE_TYPE=n1-standard-8
FIDDLE_SOURCE_SNAPSHOT=skia-systemd-pushable-base
FIDDLE_SCOPES='https://www.googleapis.com/auth/devstorage.full_control'

CURRENT=`gcloud compute instances list skia-fiddle2 --uri`
if [ "$CURRENT" == "" ]; then
  set +e
  # Create a boot disk from the pushable base snapshot.
  gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
    --zone $ZONE \
    --source-snapshot $FIDDLE_SOURCE_SNAPSHOT \
    --type "pd-standard"

  # The cmd may fail if the disk already exists, which is fine.
  # Create a large data disk.
  gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME"-data" \
    --size "1000" \
    --zone $ZONE \
    --type "pd-ssd"
  set -e

  # Create the instance with the two disks attached.
  gcloud beta compute --project $PROJECT_ID instances create $INSTANCE_NAME \
    --zone $ZONE \
    --machine-type $FIDDLE_MACHINE_TYPE \
    --network "default" \
    --maintenance-policy "TERMINATE" \
    --service-account "service-account-json@skia-buildbots.google.com.iam.gserviceaccount.com" \
    --scopes $FIDDLE_SCOPES \
    --accelerator "type=nvidia-tesla-k80,count=1" \
    --tags "http-server,https-server" \
    --metadata-from-file "startup-script=startup-script.sh" \
    --metadata "owner_primary=jcgregorio" \
    --disk "name=${INSTANCE_NAME},device-name=${INSTANCE_NAME},mode=rw,boot=yes,auto-delete=yes" \
    --disk "name=${INSTANCE_NAME}-data,device-name=${INSTANCE_NAME}-data,mode=rw,boot=no"

  sleep 20
else
  echo "Instance is already running."
fi

FIDDLE_IP_ADDRESS=`gcloud compute instances describe $INSTANCE_NAME --zone=$ZONE | grep natIP | sed s/[[:space:]]*natIP:[[:space:]]*//`

# Wait until the instance is up.
until nc -w 1 -z $FIDDLE_IP_ADDRESS 22; do
    echo "Waiting for VM to come up."
    sleep 2
done

# The instance believes it is skia-systemd-snapshot-maker until it is rebooted.
echo
echo "===== Rebooting the instance ======"
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME \
  --zone $ZONE \
  --command "sudo shutdown -r +1" \
  || echo "Reboot failed; please reboot the instance manually."

# Wait for shutdown.
sleep 90

until nc -w 1 -z $FIDDLE_IP_ADDRESS 22; do
    echo "Waiting for VM to come up."
    sleep 2
done

gcloud compute copy-files ../common/format_and_mount.sh $PROJECT_USER@$INSTANCE_NAME:/tmp/format_and_mount.sh --zone $ZONE
gcloud compute copy-files ../common/safe_format_and_mount $PROJECT_USER@$INSTANCE_NAME:/tmp/safe_format_and_mount --zone $ZONE

gcloud compute copy-files install.sh $PROJECT_USER@$INSTANCE_NAME:/tmp/install.sh --zone $ZONE
# install.sh runs dpkg configure and whiptail can't handle being run over a dumb terminal.
echo Now ssh into the instance and run /tmp/install.sh
echo gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$INSTANCE_NAME --zone $ZONE
