#/bin/bash

# Creates the autoroll-email-creds secret, used for sending email from the
# autoroller.

set -e -x
source ./config.sh
source ../bash/ramdisk.sh

SECRET_NAME="webhook-request-salt"
ORIG_WD=$(pwd)

cd /tmp/ramdisk

function get() {
  gcloud compute project-info describe --project=google.com:skia-buildbots --format="value[](commonInstanceMetadata.items.$1)" > $1
}

get webhook_request_salt
kubectl create secret generic "${SECRET_NAME}" --from-file=webhook_request_salt

cd -
