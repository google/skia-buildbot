#/bin/bash

# Install velero in the skia-public cluster.

set -e -x
source ../kube/config.sh
source ../bash/ramdisk.sh

# Defines SA_NAME
source ./config.sh

BUCKET=${SA_NAME}-${CLUSTER_NAME}

cd /tmp/ramdisk

# Extract the service account key we keep stored in secrets (default
# namespace) so velero can then use it in the velero namespace).
kubectl get secret velero-backup -o json |  jq -r .data.\"cloud\" | base64 -d > credentials-velero

velero install \
  --provider gcp \
  --bucket $BUCKET \
  --secret-file ./credentials-velero

cd -
