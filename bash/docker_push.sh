set -x

ROOT=`mktemp -d`

DATETIME=`date --utc "+%Y-%m-%dT%H:%M:%SZ"`
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

TAG=${DATETIME}/${USER}/${HASH}/${REPO_STATE}

copy_release_files

docker build -f ./docker/Dockerfile -t ${APPNAME} ${ROOT}
docker tag ${APPNAME} gcr.io/skia-public/${APPNAME}:${TAG}
docker push gcr.io/skia-public/${APPNAME}:${TAG}
echo gcr.io/skia-public/${APPNAME}:${TAG}
