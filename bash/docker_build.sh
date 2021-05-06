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

set -x -e

# Useful variables used by build_* scripts.
INSTALL="install -D --verbose --backup=none"
INSTALL_DIR="install -d --verbose --backup=none"

if [ -z "$ROOT" ]; then
  ROOT=`mktemp -d`
fi
mkdir -p ${ROOT}
PROJECT="${PROJECT:-skia-public}"
DATETIME=`date --utc "+%Y-%m-%dT%H_%M_%SZ"`
HASH=`git rev-parse HEAD`

# Determine repo state.
REPO_STATE=clean
# Detect if we have unchecked in local changes, or if we're not on the main
# branch (possibly at an older revision).
git fetch
# diff-index requires update-index --refresh; see:
# https://stackoverflow.com/questions/36367190/git-diff-files-output-changes-after-git-status/36439778#36439778
if git update-index --refresh ; then
  if ! git diff-index --quiet HEAD -- ; then
    REPO_STATE=dirty
    echo "Setting DIRTY=true due to modified files:"
    echo "$(git diff-index --name-status HEAD --)"
  elif ! git merge-base --is-ancestor HEAD origin/main ; then
    REPO_STATE=dirty
    echo "Setting DIRTY=true due to current branch: " \
      "$(git rev-parse --abbrev-ref HEAD)"
  fi
else
  echo "Setting DIRTY=true due to checked out files."
  REPO_STATE=dirty
fi

# Calculate the tag.
if [ -z "$TAG" ]; then
  # If the format of this ever changes then please also update k8s_checker/main.go
  TAG=${DATETIME}-${USER}-${HASH:0:7}-${REPO_STATE}
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
