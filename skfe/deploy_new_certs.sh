#!/bin/bash

# A script to automate some parts of new certificate deployment.
#
# This script adds a new cert to gcloud, and makes a local change to the
# skia-ingress.yaml file. Applying the change to kubernetes and pushing the
# change in git are left as manual steps to allow for manual inspection of the
# change.

DATE=$(date +%Y-%m-%d)

gcloud compute ssl-certificates create skia-org-$DATE \
    --private-key=$HOME/ssl-cert-requests/WILDCARD.skia.org-$DATE/WILDCARD.skia.org.key \
    --certificate=$HOME/ssl-cert-requests/WILDCARD.skia.org-$DATE/WILDCARD.skia.org.chained.pem \
    --project=skia-public

git clone https://skia.googlesource.com/skia-public-config

sed --in-place s#ingress.gcp.kubernetes.io/pre-shared-cert:.*#ingress.gcp.kubernetes.io/pre-shared-cert:\ skia-org-$DATE# ./skia-public-config/skia-ingress.yaml

printf "Now apply the modified yaml file:\n\n"
printf "  kubectl apply -f ./skia-public-config/skia-ingress.yaml\n\n"
printf "Don't forget to commit and push the updated config file.\n\n"

cd skia-public-config; git diff

printf "Also remove unused certs: \n\n"
gcloud compute ssl-certificates list --project=skia-public --format=json | jq '.[].name' | | grep --invert-match skia-org-$DATE | xargs -L1 echo gcloud compute ssl-certificates delete --project=skia-public