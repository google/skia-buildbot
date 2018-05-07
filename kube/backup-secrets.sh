#/bin/bash

# Backup all the kubernetes secrets to Google Cloud Storage after encrypting
# them with KMS. The secrets never touch disk and are only passed through
# pipes until they are encrypted and written to GCS.

# Note that this script requires that 'jq' be installed.
KEYRING=kubernetes-keyring
KEY=kubernetes-secrets
BUCKET=skia-public-backup

for secret in $(kubectl get secrets -o json | jq -r '.items[].metadata.name'); do
  echo "Backing up" ${secret}
  kubectl get secret ${secret} -o json | gcloud kms encrypt \
      --keyring=${KEYRING} \
      --key=${KEY} \
      --location=global \
      --plaintext-file=- \
      --ciphertext-file=- | gsutil cp - gs://${BUCKET}/secrets/$(date +%Y-%m-%d)/${secret}.enc
done
