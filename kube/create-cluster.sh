#/bin/bash

set -x -e

# Your project ID
PROJECT_ID=skia-public

# Your Zone. E.g. us-west1-c
ZONE=us-central1-a

# New service account we will create. Can be any string that isn't an existing service account. E.g. min-priv-sa
SA_NAME=skia-public-k8s

# Name for your cluster we will create or modify. E.g. example-secure-cluster
CLUSTER_NAME=skia-public

gcloud iam service-accounts create "${SA_NAME}" \
  --display-name="${SA_NAME}"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role roles/logging.logWriter

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role roles/monitoring.metricWriter

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role roles/monitoring.viewer

# TODO(jcgregorio) Add service account as reader of docker images bucket.

# TODO(jcgregorio) Also create a pool of high-cpu machines for Gold that is autoscaling.
gcloud container clusters create "${CLUSTER_NAME}" \
  --service-account="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --addons HorizontalPodAutoscaling,HttpLoadBalancing \
  --cluster-version "1.9.6-gke.1" \
  --disk-size "100" \
  --enable-autoscaling \
  --enable-autoupgrade \
  --enable-autoupgrade \
  --enable-cloud-logging \
  --enable-cloud-monitoring \
  --image-type "COS" \
  --machine-type "n1-standard-8" \
  --maintenance-window "07:00" \
  --min-nodes "3" --max-nodes "100" \
  --network "default" \
  --no-enable-basic-auth \
  --no-enable-legacy-authorization \
  --num-nodes "3" \
  --subnetwork "default" \
  --zone "us-central1-a"
