#/bin/bash

for secret in $(kubectl get secrets -o json | jq -r '.items[].metadata.name'); do
  kubectl get secret ${secret} -o yaml | gcloud kms encrypt \
      --keyring=kubernetes-keyring \
      --key=kubernetes-secrets \
      --location=global \
      --plaintext-file=- \
      --ciphertext-file=- | gsutil cp - gs://skia-public-backup/secrets/$(date +%Y-%m-%d)/${secret}.enc
done
