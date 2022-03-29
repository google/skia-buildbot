#/bin/bash

# Creates the service account for cherrypicks-watcher.

../../kube/secrets/add-service-account.sh \
  google.com:skia-corp \
  skia-corp \
  skia-cherrypicks-watcher \
  "Service account for Cherrypicks Watcher."

gcloud projects add-iam-policy-binding skia-firestore --member serviceAccount:skia-cherrypicks-watcher@skia-corp.google.com.iam.gserviceaccount.com --role roles/datastore.user
