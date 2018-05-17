#/bin/bash

set -x -e

# Add the 'latest' tag to the most recently built skiaserve image.
DIGEST=`gcloud container images list-tags gcr.io/skia-public/skiaserve --limit=1 --format=json | jq -r '.[0].digest'`
gcloud container images add-tag gcr.io/skia-public/skiaserve@${DIGEST} gcr.io/skia-public/skiaserve:latest
