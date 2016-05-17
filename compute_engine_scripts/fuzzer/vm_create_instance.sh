#!/bin/bash
#
# Creates the compute instance for skia-fuzzer.
#
set -x

source vm_config.sh

# Create a boot disk from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $FUZZER_SOURCE_SNAPSHOT \
  --type "pd-standard"

# local-ssd is 375 GB
gcloud compute --project $PROJECT_ID instances create $INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $FUZZER_MACHINE_TYPE \
  --local-ssd interface=SCSI \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $FUZZER_SCOPES \
  --tags "http-server,https-server" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --metadata "owner_primary=kjlubick,owner_secondary=jcgregorio" \
  --disk "name=${INSTANCE_NAME},device-name=${INSTANCE_NAME},mode=rw,boot=yes,auto-delete=yes" \
  --address $FUZZER_IP_ADDRESS

# Wait until the instance is up.
until nc -w 1 -z $FUZZER_IP_ADDRESS 22; do
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
