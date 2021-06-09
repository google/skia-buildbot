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

function __skia_switchboard() {
  gcloud config set project skia-switchboard > /dev/null 2>&1
  gcloud container clusters get-credentials skia-switchboard --zone us-central1-c --project skia-switchboard > /dev/null 2>&1
}
