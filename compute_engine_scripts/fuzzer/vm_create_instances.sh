#!/bin/bash
#
# Creates the compute instance for skia-fuzzer.
#
set -x

source vm_config.sh

# Create boot disks from the pushable base snapshot.
for name in "${ALL_FUZZER_INSTANCE_NAMES[@]}" ; do
  gcloud compute --project $PROJECT_ID disks create $name \
    --zone $ZONE \
    --source-snapshot $FUZZER_SOURCE_SNAPSHOT \
    --type "pd-ssd"
done

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
i=0
for name in "${FUZZER_BE_INSTANCE_NAMES[@]}" ; do
  gcloud compute --project $PROJECT_ID instances create $name \
    --zone $ZONE \
    --machine-type $FUZZER_BE_MACHINE_TYPE \
    --local-ssd interface=SCSI \
    --network "default" \
    --maintenance-policy "MIGRATE" \
    --scopes $FUZZER_SCOPES \
    --tags "http-server,https-server" \
    --metadata-from-file "startup-script=startup-script.sh" \
    --metadata "owner_primary=kjlubick,owner_secondary=jcgregorio" \
    --disk "name=${name},device-name=${name},mode=rw,boot=yes,auto-delete=yes" \
    --address ${FUZZER_BE_IP_ADDRESSES[i]}

    i=$((i+1))
done


# Wait until the instances are up.
for address in "${ALL_FUZZER_IP_ADDRESSES[@]}" ; do
  until nc -w 1 -z $address 22; do
    echo "Waiting for VM at ${address} to come up."
    sleep 2
  done
done

# run install script
for name in "${ALL_FUZZER_INSTANCE_NAMES[@]}" ; do
  gcloud compute copy-files install.sh $PROJECT_USER@$name:/tmp/install.sh --zone $ZONE

  gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$name \
    --zone $ZONE \
    --command "/tmp/install.sh" \
    || echo "Installation failure on ${name}."

    # The instance believes it is skia-systemd-snapshot-maker until it is rebooted.
    echo
    echo "===== Rebooting ${name} ======"

    # Using "shutdown -r +1" rather than "reboot" so that the connection isn't
    # terminated immediately, which causes a non-zero exit code.
    gcloud compute --project $PROJECT_ID ssh $PROJECT_USER@$name \
      --zone $ZONE \
      --command "sudo shutdown -r +1" \
      || echo "Reboot of ${name} failed; please reboot the instance manually."
done
