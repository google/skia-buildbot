# Functions to switch between GCE and GKE projects.

set -e

function __skia_public() {
  gcloud config set project skia-public > /dev/null 2>&1
  gcloud container clusters get-credentials skia-public --zone us-central1-a --project skia-public > /dev/null 2>&1
}

function __skia_corp() {
  gcloud config set project google.com:skia-corp > /dev/null 2>&1
  gcloud container clusters get-credentials skia-corp --zone us-central1-a --project google.com:skia-corp > /dev/null 2>&1
}
