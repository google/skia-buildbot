#/bin/bash

# Creates the service account for Android compile server in skia-corp.

set -e -x
source ../kube/corp-config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME="skia-android-compile"

cd /tmp/ramdisk

# Create the service account in skia-corp and add as a secret to the cluster.
__skia_corp

gcloud iam service-accounts create "${SA_NAME}" --display-name="Service account for Android compile server"

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

# Give the service account editor access to PubSub and Storage.
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/pubsub.editor --role roles/storage.admin

cd -
