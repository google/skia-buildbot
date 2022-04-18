#/bin/bash

# Creates the service account for NPM audit mirror.

../../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  skia-npm-audit-mirror \
  "Service account for NPM audit mirror."

gcloud projects add-iam-policy-binding skia-firestore --member serviceAccount:skia-npm-audit-mirror@skia-public.iam.gserviceaccount.com --role roles/datastore.user
