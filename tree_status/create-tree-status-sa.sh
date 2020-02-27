#/bin/bash

# Creates the service account for tree status.

set -e -x
source ../kube/config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME=skia-tree-status

cd /tmp/ramdisk

gcloud iam service-accounts create "${SA_NAME}" --display-name="tree-status service account"

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/datastore.user

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -
