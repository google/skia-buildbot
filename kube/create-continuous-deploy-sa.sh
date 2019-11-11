#/bin/bash

# Create the service account that has access to the skia-public-config repo
# and export a key for it into the kubernetes cluster as a secret.

set -e -x
source ./config.sh

# New service account we will create.
SA_NAME=skia-continuous-deploy

gcloud iam service-accounts create "${SA_NAME}" --display-name="Read-write access to the skia-public-config repo."

gcloud beta iam service-accounts keys create /dev/stdout --iam-account="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com" | ./secrets/add-secret-from-stdin.sh ${PROJECT_ID} ${SA_NAME}
#kubectl create secret generic "${SA_NAME}" --from-file=key.json=${SA_NAME}.json

#cd -
