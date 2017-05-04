#!/bin/bash
#
# Upgrade golang on Linux GCE Swarming bots.

source vm_config.sh # GO_VERSION comes from here.

function upgrade_go {
  echo
  echo "Upgrade Go on $1"
  gcloud compute --project $PROJECT_ID ssh --zone $ZONE ${PROJECT_USER}@$1 -- \
      "cd /tmp && " \
      "wget https://storage.googleapis.com/golang/$GO_VERSION.tar.gz && " \
      "tar -zxvf $GO_VERSION.tar.gz && " \
      "sudo rm -rf /usr/local/$GO_VERSION && " \
      "sudo mv go /usr/local/$GO_VERSION && " \
      "sudo rm -rf /usr/local/go && " \
      "sudo rm -f /usr/bin/go && " \
      "sudo ln -s /usr/local/$GO_VERSION /usr/local/go && " \
      "sudo ln -s /usr/local/$GO_VERSION/bin/go /usr/bin/go && " \
      "rm $GO_VERSION.tar.gz" \
      || FAILED="$FAILED InstallGo"
  echo
}

for MACHINE_IP in $(seq $VM_BOT_COUNT_START $VM_BOT_COUNT_END); do
  INSTANCE_NAME=${VM_BOT_NAME}-`printf "%03d" ${MACHINE_IP}`
  upgrade_go $INSTANCE_NAME
done
