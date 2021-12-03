#/bin/bash

# Creates the service account used by the Task Driver server, and export a key
# for it into the kubernetes cluster as a secret.

set -e -x
source ../kube/config.sh
source ../bash/ramdisk.sh

# New service account we will create.
SA_NAME="task-driver"
SA_EMAIL="${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com"

cd /tmp/ramdisk

gcloud --project=${PROJECT_ID} iam service-accounts create "${SA_NAME}" --display-name="Service account for Task Driver"
gcloud projects add-iam-policy-binding skia-swarming-bots --member serviceAccount:${SA_EMAIL} --role roles/pubsub.admin --role roles/logging.admin
gcloud projects add-iam-policy-binding skia-public --member serviceAccount:${SA_EMAIL} --role roles/bigtable.user
gcloud projects add-iam-policy-binding --project ${PROJECT} \
  --member serviceAccount:${SA_EMAIL} --role roles/cloudtrace.agent

cd -
