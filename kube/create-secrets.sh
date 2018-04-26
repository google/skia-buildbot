#/bin/bash

set -e -x

function finish {
  cd -
  sleep 1
  sudo umount /tmp/ramdisk
  rmdir /tmp/ramdisk
}
trap finish EXIT

# Your project ID
PROJECT_ID=skia-public

# Your Zone. E.g. us-west1-c
ZONE=us-central1-a

# New service account we will create.
SA_NAME=skia-public-auth

mkdir /tmp/ramdisk
sudo mount  -t tmpfs -o size=10m tmpfs /tmp/ramdisk
cd /tmp/ramdisk

gcloud iam service-accounts create "${SA_NAME}" --display-name="Read-only access to https://chrome-infra-auth.appspot.com API"

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

