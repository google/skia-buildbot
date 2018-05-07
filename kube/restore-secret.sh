#/bin/bash

# Restore a kubernetes secret from Google Cloud Storage.

# The argument to this script must be the full GCS path to the encrypted
# secret to restore:
#
# For example:
#
#   ./restore-secret.sh gs://skia-public-backup/secrets/2018-05-05/alertmanager-webhook-chat-config.enc
if [ "$#" -ne 1 ]; then
  echo "The argument must be a GCS path to the encrypted secret to restore.  For example:"
  echo ""
  echo "./restore-secret.sh gs://skia-public-backup/secrets/2018-05-05/alertmanager-webhook-chat-config.enc"
  exit 1
fi

gsutil cp $1 - | gcloud kms decrypt \
  --ciphertext-file=- \
  --plaintext-file=- \
  --location=global \
  --keyring=kubernetes-keyring \
  --key=kubernetes-secrets | kubectl apply -f -
