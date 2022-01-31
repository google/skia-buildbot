# This bash file is intended to be used for docker images.  To use this file
# just create your own bash file in which you define the APPNAME var and the
# copy_release_files() function which copies all the files needed in the
# distribution in ${ROOT}. At a minimum, copy_release_files() must place
# a 'Dockerfile' file immediately under ${ROOT}. Then source this file after
# those definitions. The resulting docker image will be uploaded to the Google
# Container registry.
#
# PROJECT
# -------
# If PROJECT is set then it is used as the default GCE project for the Google
# Container Registry. This value defaults to 'skia-public'. The project must
# have Google Container Registry activated.
#
# TAG
# ---
# If TAG is set then it is used as the tag for the docker image, otherwise a
# unique tag is generated from the time/date, user, git hash and repo state.
# This should never be set for application images, i.e. ones that will
# participate in pushk, which expects the auto generated tag format.
#
# SKIP_UPLOAD
# -----------
# If SKIP_UPLOAD is set then do not push the image to the container registry.
# This is useful when developing locally and needing to rapidly iterate on
# the image.
#
# SKIP_BUILD
# -----------
# If SKIP_BUILD is set then do not run docker on the ROOT directory. This also
# skips the upload step since nothing will have been built. Useful for cloud
# builder steps.
#
# ROOT
# ----
# If ROOT is not set then it will be set to a temp directory that is created,
# otherewise ROOT is presumed to exist.

set -e

# Useful variables used by build_* scripts.
INSTALL="install -D --verbose --backup=none"
INSTALL_DIR="install -d --verbose --backup=none"
REL=$(dirname "$BASH_SOURCE")

if [ -z "$ROOT" ]; then
  ROOT=`mktemp -d`
fi
mkdir -p ${ROOT}
PROJECT="${PROJECT:-skia-public}"

# Calculate the tag.
if [ -z "$TAG" ]; then
  # If the format of this ever changes then please also update k8s_checker/main.go
  TAG=`${REL}/release_tag.sh`
fi

copy_release_files

if [ -z "$SKIP_BUILD" ]; then
docker build -t ${APPNAME} ${ROOT}

  if [ -z "$SKIP_UPLOAD" ]; then
    docker tag ${APPNAME} gcr.io/${PROJECT}/${APPNAME}:${TAG}
    docker push gcr.io/${PROJECT}/${APPNAME}:${TAG}
    echo gcr.io/${PROJECT}/${APPNAME}:${TAG}
  fi
fi
