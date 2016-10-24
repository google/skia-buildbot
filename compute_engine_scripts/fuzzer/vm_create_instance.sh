#!/bin/bash
#
# Creates the compute instance for skia-fuzzer.
#
set -x

source vm_config.sh

# Create boot disks from the pushable base snapshot.
gcloud compute --project $PROJECT_ID disks create $FUZZER_FE_INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $FUZZER_SOURCE_SNAPSHOT \
  --type "pd-ssd"

gcloud compute --project $PROJECT_ID disks create $FUZZER_BE1_INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $FUZZER_SOURCE_SNAPSHOT \
  --type "pd-ssd"

gcloud compute --project $PROJECT_ID disks create $FUZZER_BE2_INSTANCE_NAME \
  --zone $ZONE \
  --source-snapshot $FUZZER_SOURCE_SNAPSHOT \
  --type "pd-ssd"

# create frontend instance
# local-ssd is 375 GB
gcloud compute --project $PROJECT_ID instances create $FUZZER_FE_INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $FUZZER_FE_MACHINE_TYPE \
  --local-ssd interface=SCSI \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $FUZZER_SCOPES \
  --tags "http-server,https-server" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --metadata "owner_primary=kjlubick,owner_secondary=jcgregorio" \
  --disk "name=${FUZZER_FE_INSTANCE_NAME},device-name=${FUZZER_FE_INSTANCE_NAME},mode=rw,boot=yes,auto-delete=yes" \
  --address $FUZZER_FE_IP_ADDRESS

# create backend instances
gcloud compute --project $PROJECT_ID instances create $FUZZER_BE1_INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $FUZZER_BE_MACHINE_TYPE \
  --local-ssd interface=SCSI \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $FUZZER_SCOPES \
  --tags "http-server,https-server" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --metadata "owner_primary=kjlubick,owner_secondary=jcgregorio" \
  --disk "name=${FUZZER_BE1_INSTANCE_NAME},device-name=${FUZZER_BE1_INSTANCE_NAME},mode=rw,boot=yes,auto-delete=yes" \
  --address $FUZZER_BE1_IP_ADDRESS

gcloud compute --project $PROJECT_ID instances create $FUZZER_BE2_INSTANCE_NAME \
  --zone $ZONE \
  --machine-type $FUZZER_BE_MACHINE_TYPE \
  --local-ssd interface=SCSI \
  --network "default" \
  --maintenance-policy "MIGRATE" \
  --scopes $FUZZER_SCOPES \
  --tags "http-server,https-server" \
  --metadata-from-file "startup-script=startup-script.sh" \
  --metadata "owner_primary=kjlubick,owner_secondary=jcgregorio" \
  --disk "name=${FUZZER_BE2_INSTANCE_NAME},device-name=${FUZZER_BE2_INSTANCE_NAME},mode=rw,boot=yes,auto-delete=yes" \
  --address $FUZZER_BE2_IP_ADDRESS

# Wait until the instances are up.
until nc -w 1 -z $FUZZER_FE_IP_ADDRESS 22; do
    echo "Waiting for FE VM to come up."
    sleep 2
done

until nc -w 1 -z $FUZZER_BE1_IP_ADDRESS 22; do
    echo "Waiting for BE1 VM to come up."
    sleep 2
done

until nc -w 1 -z $FUZZER_BE2_IP_ADDRESS 22; do
    echo "Waiting for BE2 VM to come up."
    sleep 2
done


gcloud compute copy-files install.sh $PROJECT_USER@$FUZZER_FE_INSTANCE_NAME:/tmp/install.sh --zone $ZONE
gcloud compute copy-files install.sh $PROJECT_USER@$FUZZER_BE1_INSTANCE_NAME:/tmp/install.sh --zone $ZONE
gcloud compute copy-files install.sh $PROJECT_USER@$FUZZER_BE2_INSTANCE_NAME:/tmp/install.sh --zone $ZONE

gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$FUZZER_FE_INSTANCE_NAME \
  --zone $ZONE \
  --command "/tmp/install.sh" \
  || echo "Installation failure."

gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$FUZZER_BE1_INSTANCE_NAME \
  --zone $ZONE \
  --command "/tmp/install.sh" \
  || echo "Installation failure."

gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$FUZZER_BE2_INSTANCE_NAME \
  --zone $ZONE \
  --command "/tmp/install.sh" \
  || echo "Installation failure."


# The instance believes it is skia-systemd-snapshot-maker until it is rebooted.
echo
echo "===== Rebooting the instances ======"

# Using "shutdown -r +1" rather than "reboot" so that the connection isn't
# terminated immediately, which causes a non-zero exit code.
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$FUZZER_FE_INSTANCE_NAME \
  --zone $ZONE \
  --command "sudo shutdown -r +1" \
  || echo "Reboot failed; please reboot the instance manually."

# Using "shutdown -r +1" rather than "reboot" so that the connection isn't
# terminated immediately, which causes a non-zero exit code.
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$FUZZER_BE1_INSTANCE_NAME \
  --zone $ZONE \
  --command "sudo shutdown -r +1" \
  || echo "Reboot failed; please reboot the instance manually."

# Using "shutdown -r +1" rather than "reboot" so that the connection isn't
# terminated immediately, which causes a non-zero exit code.
gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$FUZZER_BE2_INSTANCE_NAME \
  --zone $ZONE \
  --command "sudo shutdown -r +1" \
  || echo "Reboot failed; please reboot the instance manually."