#!/bin/bash
set -e
set -o pipefail

REL=$(dirname "$0")
source ${REL}/config.sh

# name="chromium-swarm-bots@skia-swarming-bots.iam.gserviceaccount.com-sa-key"
# if [[ "$name" == *"iam.gserviceaccount.com"* ]]; then
#   echo "match"
#   exit 1
# fi
# exit 1

while read -r line; do
  # Derive the GCP secret name.
  srcSecretName="$(echo "$line" | awk '{print $1;}')"
  srcSecretBaseName="$(echo $srcSecretName | cut -d "/" -f 2)"
  dstSecretName="$(berglas_to_gcp_secret_name ${srcSecretName})"

  # Skip skia-corp; it no longer exists.
  cluster="$(echo $srcSecretName | cut -d "/" -f 1)"
  if [[ "$cluster" == "skia-corp" ]]; then
    echo "$srcSecretName is unused (skia-corp); skipping"
    continue
  fi

  # Skip secrets that are full service account email addresses; I'm convinced
  # they were created by accident.
  if [[ "$dstSecretName" == *"iam.gserviceaccount.com"* ]]; then
    echo "$srcSecretName looks like a full email; skipping"
    continue
  fi

  # Skip secrets that already exist.
  gcloud --project=skia-infra-public secrets describe ${dstSecretName} >> /dev/null 2>&1 && \
    echo "$srcSecretName is already migrated; skipping" && \
    continue

  # Skip any secrets which aren't structured like service account keys.
  value="$(berglas access ${BUCKET_ID}/$srcSecretName | base64 -d)"
  fieldValue="$(echo "$value" | yq e '.data."key.json"')"
  if [[ "$fieldValue" == "null" ]] || [[ "$fieldValue" == "" ]]; then
    echo "$srcSecretName is not a service account key; skipping"
    continue
  fi

  # Migrate the secret.
  echo "Migrating: $srcSecretName to $dstSecretName"
  ${REL}/migrate-berglas-to-gcp.sh ${cluster} ${srcSecretBaseName} '.data."key.json"' ${dstSecretName}
  #exit 0
done <<< "$(berglas list ${BUCKET_ID} | tail -n +2)"