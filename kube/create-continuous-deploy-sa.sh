#/bin/bash

# Create the service account that has access to the skia-public-config repo
# and export a key for it into the kubernetes cluster as a secret.

set -e -x
source ./config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME=skia-continuous-deploy

cd /tmp/ramdisk

gcloud iam service-accounts create "${SA_NAME}" --display-name="Read-write access to the skia-public-config repo."

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -
