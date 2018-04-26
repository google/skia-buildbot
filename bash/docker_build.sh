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

set -x -e

ROOT=`mktemp -d`
PROJECT="${PROJECT:-skia-public}"
DATETIME=`date --utc "+%Y-%m-%dT%H_%M_%SZ"`
HASH=`git rev-parse HEAD`

# Detect if we have unchecked in local changes, or if we're not on the master
# branch (possibly at an older revision).
git fetch
# diff-index requires update-index --refresh; see:
# https://stackoverflow.com/questions/36367190/git-diff-files-output-changes-after-git-status/36439778#36439778
git update-index --refresh
REPO_STATE=clean
if ! git diff-index --quiet HEAD -- ; then
  REPO_STATE=dirty
  echo "Setting DIRTY=true due to modified files:"
  echo "$(git diff-index --name-status HEAD --)"
elif ! git merge-base --is-ancestor HEAD origin/master ; then
  REPO_STATE=dirty
  echo "Setting DIRTY=true due to current branch: " \
    "$(git rev-parse --abbrev-ref HEAD)"
fi

if [ -z "$TAG" ]; then
  TAG=${DATETIME}-${USER}-${HASH}-${REPO_STATE}
fi

copy_release_files

docker build -t ${APPNAME} ${ROOT}
docker tag ${APPNAME} gcr.io/${PROJECT}/${APPNAME}:${TAG}
docker push gcr.io/${PROJECT}/${APPNAME}:${TAG}
echo gcr.io/${PROJECT}/${APPNAME}:${TAG}
