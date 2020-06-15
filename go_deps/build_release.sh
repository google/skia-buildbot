#!/bin/bash

set -x -e

# Create and upload a container image for go_build
APPNAME=go_build

REPO_ROOT="$(git rev-parse --show-toplevel)"

# At the time of writing, this is "golang:1.14"
IMAGE="$(cat ${REPO_ROOT}/go_deps/image.sha256)"

# CIPD package information.
CIPD_URL="https://chrome-infra-packages.appspot.com"
CIPD_PKG="skia/bots/go_cache"
HASH="$(git rev-parse HEAD)"
CIPD_TAG="revision:${HASH}"
# TODO(borenet): This was copied from docker_build.sh.
REPO_STATE=clean
# Detect if we have unchecked in local changes, or if we're not on the master
# branch (possibly at an older revision).
git fetch
# diff-index requires update-index --refresh; see:
# https://stackoverflow.com/questions/36367190/git-diff-files-output-changes-after-git-status/36439778#36439778
if git update-index --refresh ; then
  if ! git diff-index --quiet HEAD -- ; then
    REPO_STATE=dirty
  elif ! git merge-base --is-ancestor HEAD origin/master ; then
    REPO_STATE=dirty
  fi
else
  REPO_STATE=dirty
fi
if [ "${REPO_STATE}" = "dirty" ]; then
  CIPD_TAG="${CIPD_TAG}_${USER}_$(date --utc "+%Y-%m-%dT%H_%M_%SZ")_dirty"
fi

TMP="$(mktemp -d)"
GOPATH="${TMP}/go"
GOCACHE="${GOPATH}/cache"
mkdir -p "${GOCACHE}"

function go_build() {
  docker run --rm \
      --user $(id -u ${USER}):$(id -g ${USER}) \
      --mount type=bind,destination=/repo,source=${REPO_ROOT},readonly \
      --mount type=bind,destination=/out,source=${GOPATH} \
      --workdir /repo \
      --env GOFLAGS=-mod=readonly \
      --env GOPATH=/out \
      --env GOCACHE=/out/cache \
      --env GOOS=$2 \
      --env GOARCH=$3 \
      ${IMAGE} \
      go $1 -v --trimpath $4
}

# Populate the GOPATH and GOCACHE.
PLATFORMS=(linux-amd64 linux-arm darwin-amd64 windows-amd64)
for PLATFORM in ${PLATFORMS[@]}; do
  GOOS="$(echo "$PLATFORM" | cut -d'-' -f1)"
  GOARCH="$(echo "$PLATFORM" | cut -d'-' -f2)"
  go_build build ${GOOS} ${GOARCH} ./...
done

# Install the "go-build" task driver. This is required because "go-build" itself
# builds task drivers, resulting in a chicken-and-egg problem.
go_build install linux amd64 ./infra/bots/task_drivers/go_build

echo "Wrote ${TMP}"

# Upload the CIPD package.
cipd create \
    --service-url ${CIPD_URL} \
    --name ${CIPD_PKG} \
    --in ${TMP} \
    --tag ${CIPD_TAG} \
    --compression-level 1 \
    --verification-timeout 30m0s

echo ${CIPD_TAG} > ${REPO_ROOT}/go_deps/pkg.version

rm -rf ${TMP}

# Copy files into the right locations in ${ROOT}.
copy_release_files()
{
INSTALL="install -D --verbose --backup=none"
INSTALL_DIR="install -d --verbose --backup=none"

# Add the dockerfile and binary.
${INSTALL} --mode=444 -T ../go.mod     ${ROOT}/go.mod
${INSTALL} --mode=444 -T ../go.sum     ${ROOT}/go.sum
${INSTALL} --mode=644 -T ./Dockerfile  ${ROOT}/Dockerfile
for script in $(ls install*.sh); do
  ${INSTALL} --mode=755 -T ./${script} ${ROOT}/${script}
done
}

#source ../bash/docker_build.sh
#docker inspect --format='{{index .RepoDigests 0}}' go_deps > image.sha256