#!/bin/bash

# Initializes the berglas secret storage.

source ./config.sh

gcloud services enable --project ${PROJECT_ID} \
  cloudkms.googleapis.com \
  storage-api.googleapis.com \
  storage-component.googleapis.com

berglas bootstrap --project $PROJECT_ID --bucket $BUCKET_ID