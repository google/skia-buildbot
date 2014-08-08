#!/bin/bash
#
# Create and setup the Skia RecreateSKPs GCE instance.
#
# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh
source vm_setup_utils.sh

# Set OS specific GCE variables.
if [ "$VM_INSTANCE_OS" == "Linux" ]; then
  SKIA_BOT_IMAGE_NAME=$SKIA_BOT_LINUX_IMAGE_NAME
  REQUIRED_FILES_FOR_BOTS=${REQUIRED_FILES_FOR_LINUX_BOTS[@]}
  WAIT_TIME_AFTER_CREATION_SECS=600
elif [ "$VM_INSTANCE_OS" == "Windows" ]; then
  SKIA_BOT_IMAGE_NAME=$SKIA_BOT_WIN_IMAGE_NAME

  ORIG_STARTUP_SCRIPT="../../scripts/win_setup.ps1"
  MODIFIED_STARTUP_SCRIPT="/tmp/win_setup.ps1"
  # Set chrome-bot's password in win_setup.ps1
  cp $ORIG_STARTUP_SCRIPT $MODIFIED_STARTUP_SCRIPT
  sed -i "s/CHROME_BOT_PASSWORD/$(echo $(cat /tmp/gce.txt) | sed -e 's/[\/&]/\\&/g')/g" $MODIFIED_STARTUP_SCRIPT
  sed -i "s/GS_ACCESS_KEY_ID/$(echo $(cat ~/.boto | sed -n 2p) | sed -e 's/[\/&]/\\&/g')/g" $MODIFIED_STARTUP_SCRIPT
  sed -i "s/GS_SECRET_ACCESS_KEY/$(echo $(cat ~/.boto | sed -n 3p) | sed -e 's/[\/&]/\\&/g')/g" $MODIFIED_STARTUP_SCRIPT

  METADATA_ARGS="--metadata=gce-initial-windows-user:chrome-bot \
                 --metadata_from_file=gce-initial-windows-password:/tmp/chrome-bot.txt \
                 --metadata_from_file=sysprep-oobe-script-ps1:$MODIFIED_STARTUP_SCRIPT"
  DISK_ARGS="--boot_disk_size_gb=$PERSISTENT_DISK_SIZE_GB"
  REQUIRED_FILES_FOR_BOTS=${REQUIRED_FILES_FOR_WIN_BOTS[@]}
  # We have to wait longer for windows because sysprep can take a while to
  # complete.
  WAIT_TIME_AFTER_CREATION_SECS=900
else
  echo "$VM_INSTANCE_OS is not recognized!"
  exit 1
fi

# Check that all required files exist.
for REQUIRED_FILE in ${REQUIRED_FILES_FOR_BOTS[@]}; do
  if [ ! -f $REQUIRED_FILE ]; then
    echo "Please create $REQUIRED_FILE!"
    exit 1
  fi
done

# Create all requested instances.
for MACHINE_IP in $(seq $VM_BOT_COUNT_START $VM_BOT_COUNT_END); do
  INSTANCE_NAME=${VM_BOT_NAME}-`printf "%03d" ${MACHINE_IP}`

  if [ "$VM_INSTANCE_OS" == "Linux" ]; then
    # The persistent disk of linux GCE bots is based on the bot's IP address.
    DISK_ARGS=--disk=skia-disk-`printf "%03d" ${MACHINE_IP}`
  fi

  $GCOMPUTE_CMD addinstance ${INSTANCE_NAME} \
    --zone=$ZONE \
    --external_ip_address=${IP_ADDRESS_WITHOUT_MACHINE_PART}.${MACHINE_IP} \
    --service_account=$PROJECT_USER \
    --service_account_scopes="$SCOPES" \
    --network=$SKIA_NETWORK_NAME \
    --image=$SKIA_BOT_IMAGE_NAME \
    --machine_type=$SKIA_BOT_MACHINE_TYPE \
    --auto_delete_boot_disk \
    --wait_until_running \
    $DISK_ARGS $METADATA_ARGS

  if [ $? -ne 0 ]; then
    echo
    echo "===== There was an error creating ${INSTANCE_NAME}. ====="
    echo
    exit 1
  fi
done

echo
echo "===== Wait $WAIT_TIME_AFTER_CREATION_SECS secs for all instances to" \
     "come up. ====="
echo
sleep $WAIT_TIME_AFTER_CREATION_SECS

# Looping through all bots and setting them up.
for MACHINE_IP in $(seq $VM_BOT_COUNT_START $VM_BOT_COUNT_END); do
  INSTANCE_NAME=${VM_BOT_NAME}-`printf "%03d" ${MACHINE_IP}`

  if [ "$VM_INSTANCE_OS" == "Linux" ]; then
    FAILED=""

    install_packages

    setup_symlinks

    checkout_skia_repos

    setup_android_sdk

    setup_nacl

    fix_gsutil_path

    copy_files

    if [ "$VM_IS_BUILDBOT" = True ]; then
      setup_crontab

      reboot
    fi

    if [[ $FAILED ]]; then
      echo
      echo "FAILURES: $FAILED"
      echo "Please manually fix these errors."
      echo
    fi

  elif [ "$VM_INSTANCE_OS" == "Windows" ]; then
    # Restart the windows instance to run chrome-bot's scheduled task.
    $GCOMPUTE_CMD resetinstance $INSTANCE_NAME
  fi
done

cat <<INP

Instances are ready to use.

INP
