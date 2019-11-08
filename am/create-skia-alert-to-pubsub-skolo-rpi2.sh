#/bin/bash

# Creates the service account for alert-to-pubsub in skia-corp.

set -e -x
source ../kube/config-only.sh
source ../bash/ramdisk.sh

# New service account we will create. Name must be the same for all clusters
# that alert-to-pubsub runs in.
SA_NAME=skolo-rpi2-alert-to-pubsub

cd /tmp/ramdisk

CLUSTER=$(kubectl config current-context)
if [ "$CLUSTER" != "skolo_rpi2" ]
then
  echo "Wrong cluster, must be run in skolo_rpi2."
  exit 1
fi

gcloud config set project skia-public
gcloud iam service-accounts create "${SA_NAME}" --display-name="alert-to-pubsub service account."

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

# Give the service account editor access to PubSub.

gcloud projects add-iam-policy-binding skia-public  \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/pubsub.editor

cd -
