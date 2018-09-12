# Build a release of gcr.io/skia-public/infra:prod.
#
# This is normally done by Cloud Builder. Use this file to manually create an image.
#
set -e -x
source ./kube/config.sh

cd docker
docker build -t gcr.io/skia-public/infra:prod .
docker push gcr.io/skia-public/infra:prod
