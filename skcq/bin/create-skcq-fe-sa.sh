#/bin/bash

# Creates the service account for SkCQ Frontend.

set -e -x
source ../../kube/config.sh

# New service account we will create.
SA_NAME="skcq-fe"
SA_EMAIL="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

../../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  ${SA_NAME} \
  "Service account for SkCQ Frontend."
gcloud projects add-iam-policy-binding skia-firestore --member serviceAccount:${SA_EMAIL} --role roles/datastore.user
