#/bin/bash
# Creates the service account for hashtag.

set -e -x

source ../kube/config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME=skia-hashtag

cd /tmp/ramdisk
gcloud iam service-accounts create "${SA_NAME}" --display-name="Skia Hashtag Service Account"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/datastore.user

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com \
  --role roles/cloudtrace.agent

gcloud projects add-iam-policy-binding skia-firestore \
  --member serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com \
  --role roles/datastore.user

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json
cd -

# Should be added to Monorail api_clients list:
#    https://bugs.chromium.org/p/monorail/issues/detail?id=6529