#/bin/bash
# Creates the service account for perf-ingest.

set -e -x

source ../kube/config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME=perf-ingest
cd /tmp/ramdisk
gcloud iam service-accounts create "${SA_NAME}" --display-name="perf-ingest service account"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" \
  --role roles/pubsub.editor

gcloud projects add-iam-policy-binding --project ${PROJECT} \
  --member serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com \
  --role roles/cloudtrace.agent

gsutil iam ch "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com:objectViewer" gs://skia-perf
gsutil iam ch "serviceAccount:${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com:objectViewer" gs://cluster-telemetry-perf

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"
kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json
cd -
