#!/bin/bash

# A script to automate some parts of new certificate deployment.
#
# See http://go/skia-ssl-cert for a longer description and the command you need
# to run before running this script.
#
# This script must be run with breakglass. See http://go/skia-infra-iac-handbook
# for instructions.
#
# This script adds a new cert to gcloud, and makes a local change to the
# skia-ingress.yaml file. Applying the change to kubernetes, pushing the change
# in git, and deleting old certs are left as manual steps to allow for manual
# inspection of the change.

set -e

DATE=$(date +%Y-%m-%d)
gcloud compute ssl-certificates create skia-org-$DATE \
    --private-key=$HOME/ssl-cert-requests/WILDCARD.skia.org-$DATE/WILDCARD.skia.org.key \
    --certificate=$HOME/ssl-cert-requests/WILDCARD.skia.org-$DATE/WILDCARD.skia.org.chained.pem \
    --project=skia-infra-public
gcloud compute ssl-certificates create skia-org-$DATE \
    --private-key=$HOME/ssl-cert-requests/WILDCARD.skia.org-$DATE/WILDCARD.skia.org.key \
    --certificate=$HOME/ssl-cert-requests/WILDCARD.skia.org-$DATE/WILDCARD.skia.org.chained.pem \
    --project=skia-public

git clone https://skia.googlesource.com/k8s-config

sed --in-place s#ingress.gcp.kubernetes.io/pre-shared-cert:.*#ingress.gcp.kubernetes.io/pre-shared-cert:\ skia-org-$DATE# ./k8s-config/skia-public/skia-ingress.yaml
sed --in-place s#ingress.gcp.kubernetes.io/pre-shared-cert:.*#ingress.gcp.kubernetes.io/pre-shared-cert:\ skia-org-$DATE# ./k8s-config/skia-infra-public/skia-ingress.yaml

printf "\n\nConfirm that the change to skia-ingress.yaml makes sense:\n\n"

cd k8s-config; git diff

printf "\n\nThen apply the modified yaml file after checking that you are working in skia-public:\n\n"
printf "  kubectl apply -f ./k8s-config/skia-public/skia-ingress.yaml\n\n"
printf "And commit and push the updated config file.\n\n"
printf "  cd k8s-config; git add --all; git commit -m 'Update skia.org certs on $DATE'; \n\n"
printf "  git cl upload --skip-title --reviewers=\"rubber-stamper@appspot.gserviceaccount.com\" --enable-auto-submit --send-mail --force\n\n"

printf "Also remove unused certs: \n\n"
gcloud compute ssl-certificates list --project=skia-public --format=json | jq '.[].name' | grep --invert-match skia-org-$DATE | xargs -L1 echo "  " gcloud compute ssl-certificates delete --project=skia-public
gcloud compute ssl-certificates list --project=skia-infra-public --format=json | jq '.[].name' | grep --invert-match skia-org-$DATE | xargs -L1 echo "  " gcloud compute ssl-certificates delete --project=skia-infra-public
