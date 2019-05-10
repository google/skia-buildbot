#/bin/bash

source ../kube/config.sh

SA_NAME="velero-backup"

BUCKET=gs://${SA_NAME}-${CLUSTER_NAME}

touch /tmp/fakekey.json
velero install \
  --provider gcp \
  --bucket $BUCKET \
  --secret-file /tmp/fakekey.json \
  --dry-run -o yaml > velero-original.yaml

kustomize build . > velero-k.yaml
