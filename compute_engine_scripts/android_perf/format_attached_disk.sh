#!/bin/bash
#
# Creates the compute instance for skia-android-perf
#
set -x

IP_ADDRESS=`gcloud compute --project google.com:skia-buildbots instances describe skia-android-perf --zone us-central1-c | grep natIP | sed s#natIP:##g`

echo $IP_ADDRESS

# Wait until the instance is up.
until nc -w 1 -z $IP_ADDRESS 22; do
    echo "Waiting for VM to come up."
    sleep 2
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

