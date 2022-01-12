#/bin/bash

set -e -x
source ../kube/config.sh

# New service account we will create.
SA_NAME="skia-codesize"
SA_EMAIL="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

# Create service account.
gcloud --project=${PROJECT_ID} iam service-accounts create "${SA_NAME}" \
    --display-name="Service account for codesize.skia.org in skia-public"

# Allow the new service account to impersonate the Google Cloud service account. See
# https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#authenticating_to.
gcloud iam service-accounts add-iam-policy-binding \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:skia-public.svc.id.goog[default/skia-codesize]" \
  skia-codesize@skia-public.iam.gserviceaccount.com

# Grant full control over the contents of the skia-codesize GCS bucket.
gsutil iam ch "serviceAccount:${SA_EMAIL}:objectAdmin" gs://skia-codesize

# Necessary in order create PubSub subscriptions to the skia-codesize-files topic. GCS will notify
# us of changes to the gs://skia-codesize bucket via this topic.
#
# Unlike most other apps in our repo, replicas do not share a set of pre-existing subscriptions.
# Instead, Each replica creates its own unique subscription (using its hostname as a suffix for the
# subscription name) in order to prevent PubSub from load-balancing messages across replicas. This
# guarantees that all replicas will be notified when a new file is uploaded to the GCS bucket. The
# pubsub.editor role is thus needed because less permissive roles such as pubsub.subscriber or
# pubsub.viewer do not allow creating new subscriptions.
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member serviceAccount:${SA_EMAIL} --role roles/pubsub.editor
