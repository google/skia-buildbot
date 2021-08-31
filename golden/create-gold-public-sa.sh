#/bin/bash

# Creates the service account used by Gold running in K8s and export a key for
# it into the kubernetes cluster as a secret.

set -e -x
source ../kube/config.sh

# New service account we will create.
SA_NAME="skia-gold"
SA_EMAIL="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

# Not only do we need to give permission to this gold service account to access
# pubsub, we need to grant access to
# service-[PROJECT_NUMBER]@gs-project-accounts.iam.gserviceaccount.com
# https://cloud.google.com/storage/docs/projects#service-accounts
# This service account has been created for us by Cloud Storage and
# will be used when interacting with GCS bucket pubsub events.
# By default, this service account lacks the permissions to interact with
# PubSub, leading to errors like:
# The service account 'service-[PROJECT_NUMBER]@gs-project-accounts.iam.gserviceaccount.com'
# does not have permission to publish messages to to the Cloud Pub/Sub topic
# '//pubsub.googleapis.com/projects/[PROJECT_ID]/topics/gold-flutter-eventbus',
# or that topic does not exist.\"
PROJECT_NUMBER=`gcloud projects describe ${PROJECT_ID} --format 'value(projectNumber)'`
GS_SA_EMAIL="service-${PROJECT_NUMBER}@gs-project-accounts.iam.gserviceaccount.com"

gcloud --project=${PROJECT_ID} iam service-accounts create "${SA_NAME}" \
    --display-name="Service account for Skia Gold in skia-public"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member serviceAccount:${SA_EMAIL} --role roles/bigtable.user

# datastore and firestore share the same roles
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member serviceAccount:${SA_EMAIL} --role roles/datastore.user

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member serviceAccount:${SA_EMAIL} --role roles/pubsub.admin

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member serviceAccount:${SA_EMAIL} --role roles/storage.admin

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member serviceAccount:${GS_SA_EMAIL} --role roles/pubsub.editor

gcloud projects add-iam-policy-binding --project ${PROJECT} \
  --member serviceAccount:${SA_EMAIL} --role roles/cloudtrace.agent
