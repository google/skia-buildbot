#!/bin/bash
#
# Create and setup the Skia RecreateSKPs GCE instance.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh
source vm_setup_utils.sh

for REQUIRED_FILE in ${REQUIRED_FILES_FOR_SKIA_BOTS[@]}; do
  if [ ! -f $REQUIRED_FILE ];
  then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done

# Create all requested instances.
for MACHINE_IP in $(seq $VM_BOT_COUNT_START $VM_BOT_COUNT_END); do
  INSTANCE_NAME=${VM_BOT_NAME}-`printf "%03d" ${MACHINE_IP}`

  $GCOMPUTE_CMD addinstance ${INSTANCE_NAME} \
    --zone=$ZONE \
    --external_ip_address=${IP_ADDRESS_WITHOUT_MACHINE_PART}.${MACHINE_IP} \
    --service_account=$PROJECT_USER \
    --service_account_scopes="$SCOPES" \
    --network=$SKIA_NETWORK_NAME \
    --image=$SKIA_BOT_IMAGE_NAME \
    --machine_type=$SKIA_BOT_MACHINE_TYPE \
    --auto_delete_boot_disk

  if [ $? -ne 0 ]
  then
    echo
    echo "===== There was an error creating ${INSTANCE_NAME}. ====="
    echo
    exit 1
  fi
done

echo "===== Wait 3 mins for all $BOT_COUNT instances to come up. ====="
sleep 180


# Looping through all bots and setting them up.
for MACHINE_IP in $(seq $VM_BOT_COUNT_START $VM_BOT_COUNT_END); do
  INSTANCE_NAME=${VM_BOT_NAME}-`printf "%03d" ${MACHINE_IP}`

  FAILED=""

  install_packages

  setup_symlinks

  checkout_skia_repos

  setup_android_sdk

  setup_nacl

  setup_crontab

  copy_files

  if [[ $FAILED ]]; then
    echo
    echo "FAILURES: $FAILED"
    echo "Please manually fix these errors."
    echo
  fi

done

cat <<INP

Start the bots on the instances with:
* cd $SKIA_REPO_DIR/buildbot
* nohup python scripts/launch_slaves.py &

INP
