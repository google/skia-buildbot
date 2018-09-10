#/bin/bash

# Creates the autoroll-email-creds secret, used for sending email from the
# autoroller.

set -e -x
source ./config.sh
source ../bash/ramdisk.sh

SECRET_NAME="autoroll-email-creds"

cd /tmp/ramdisk

function get() {
  gcloud compute project-info describe --project=google.com:skia-buildbots --format="value[](commonInstanceMetadata.items.$1)" > $1
}

get gmail_clientid
get gmail_clientsecret
get gmail_cached_token_autoroll

kubectl create secret generic "${SECRET_NAME}" --from-file=gmail_clientid --from-file=gmail_clientsecret --from-file=gmail_cached_token_autoroll

cd -
