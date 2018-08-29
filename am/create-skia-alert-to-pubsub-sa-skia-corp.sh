#/bin/bash

# Creates the service account for alert-to-pubsub in skia-corp.

set -e -x
source ../kube/corp-config.sh
source ../bash/ramdisk.sh

# New service account we will create. Name must be the same for all clusters
# that alert-to-pubsub runs in.
SA_NAME=skia-alert-to-pubsub

cd /tmp/ramdisk

# Create the service account in skia-corp and add as a secret to the cluster.
__skia_corp

gcloud iam service-accounts create "${SA_NAME}" --display-name="alert-to-pubsub service account."

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

# Change to skia-public and give the service account editor access to PubSub.
__skia_public

gcloud projects add-iam-policy-binding skia-public  \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/pubsub.editor

__skia_corp

cd -
