#/bin/bash

# Creates the service account for SkCQ Backend.

set -e -x
source ../../kube/config.sh

# New service account we will create.
SA_NAME="skcq-be"
SA_EMAIL="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

../../kube/secrets/add-service-account.sh \
  google.com:skia-corp \
  skia-corp \
  ${SA_NAME} \
  "Service account for SkCQ Backend."
gcloud projects add-iam-policy-binding skia-firestore --member serviceAccount:${SA_EMAIL} --role roles/datastore.user
