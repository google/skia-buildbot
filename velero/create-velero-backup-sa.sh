#/bin/bash

# Creates the service account used by Velero to backup secrets. We use the
# same service account across all clusters.

set -e -x
source ../kube/config.sh
source ../bash/ramdisk.sh

# Defines SA_NAME
source ./config.sh

BUCKET=${SA_NAME}-${CLUSTER_NAME}

cd /tmp/ramdisk

gcloud --project=${PROJECT_ID} iam service-accounts create "${SA_NAME}" --display-name="Service account for Velero backup"

SA_EMAIL=${SA_NAME}@${PROJECT_SUBDOMAIN}.iam.gserviceaccount.com

ROLE_PERMISSIONS=(
     compute.disks.get
     compute.disks.create
     compute.disks.createSnapshot
     compute.snapshots.get
     compute.snapshots.create
     compute.snapshots.useReadOnly
     compute.snapshots.delete
     compute.zones.get
 )

gcloud iam roles create velero.server \
     --project $PROJECT_ID \
     --title "Velero Server" \
     --permissions "$(IFS=","; echo "${ROLE_PERMISSIONS[*]}")"

gcloud projects add-iam-policy-binding $PROJECT_ID \
     --member serviceAccount:${SA_EMAIL} \
     --role projects/$PROJECT_ID/roles/velero.server

gsutil mb -p $PROJECT_ID -c nearline gs://$BUCKET
gsutil iam ch serviceAccount:${SA_EMAIL}:objectAdmin gs://${BUCKET}

gcloud iam service-accounts keys create ${SA_NAME}.json --iam-account="${SA_EMAIL}"

kubectl create secret generic ${SA_NAME} --from-file=cloud=${SA_NAME}.json

cd -

# Create buckets for other clusters and add the right perms.

# skia-corp
source ../kube/corp-config.sh

# Defines SA_NAME, which we need to do again since corp-config.sh also defines it.
source ./config.sh
BUCKET=${SA_NAME}-${CLUSTER_NAME}

gsutil mb -p $PROJECT_ID -c nearline gs://$BUCKET
gsutil iam ch serviceAccount:${SA_EMAIL}:objectAdmin gs://${BUCKET}

# To extract the 'cloud' file run:
#
#   kubectl get secret velero-backup -o json |  jq -r .data.\"cloud\" | base64   -d > cloud
#

