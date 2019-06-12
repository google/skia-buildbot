#/bin/bash
# Creates the service account for push.

set -e -x

source ../kube/config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME=skia-push

cd /tmp/ramdisk
gcloud iam service-accounts create "${SA_NAME}" --display-name="push service account"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/compute.viewer

gsutil iam ch "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com:objectCreator" gs://skia-push

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"
kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json
cd -
