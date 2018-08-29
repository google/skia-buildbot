#/bin/bash

# Creates the service account that has read-only access to the spreadsheet
# with the contest entries. This service account has no real roles, as it
# just needs read-only access to a spreadsheet that is world readable.

set -e -x
source ../kube/config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME=skia-contest

cd /tmp/ramdisk

gcloud iam service-accounts create "${SA_NAME}" --display-name="Uses the spreadsheets API to get access to the contest spreadsheet."

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -
