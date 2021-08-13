#!/bin/bash
set -e

# Initializes the berglas secret storage.

REL=$(dirname "$0")
source ${REL}/config.sh

gcloud services enable --project ${PROJECT_ID} \
  cloudkms.googleapis.com \
  storage-api.googleapis.com \
  storage-component.googleapis.com

berglas bootstrap --project $PROJECT_ID --bucket $BUCKET_ID