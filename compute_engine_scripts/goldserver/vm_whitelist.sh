#!/bin/bash
#
# Sets the white list for an instance to determine who is allowed to log in.
# The provided white list file should contain space/newline separated email
# addresses and domain names.
#
set -x

if [ "$#" -ne 2 ]; then
    echo "Usage: <prod|stage|android|blink> white_list_file"
    exit 1
fi

VM_ID=$1
source vm_config.sh

gcloud compute --project $PROJECT_ID instances add-metadata $INSTANCE_NAME \
  --zone $ZONE \
  --metadata-from-file auth_white_list=$2
