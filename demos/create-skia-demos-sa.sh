#/bin/bash

# Creates the service account for skia-demos in skia-corp.
# We cannot yet use workload identity in the corp cluster: b/171523347

set -e -x
source ../kube/corp-config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME="skia-demos"

cd /tmp/ramdisk

# Create the service account in skia-corp and add as a secret to the cluster.
__skia_corp

gcloud iam service-accounts create "${SA_NAME}" --display-name="Service account for skia-demos server"

gcloud beta iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

cd -
