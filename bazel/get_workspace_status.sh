#!/bin/bash -e

# This script is intended to be passed to Bazel using the --workspace_status_command command-line
# flag. It defines various key/value pairs, such as the git hash or clean/dirty status, which can be
# used from BUILD files, e.g. to tag Docker images.
#
# See https://docs.bazel.build/versions/master/user-manual.html#flag--workspace_status_command.

BUILD_DATETIME=`date --utc +%Y-%m-%dT%H_%M_%SZ`
echo "BUILD_DATETIME $BUILD_DATETIME"

STABLE_GIT_REVISION=`git rev-parse HEAD`
echo "STABLE_GIT_REVISION $STABLE_GIT_REVISION"

# Check whether there are any uncommitted changes.
#
# Based on:
# https://skia.googlesource.com/buildbot/+/cdbd6dc7cd9e06604042bb53a6179a77b4c83c25/bash/docker_build.sh#53
STABLE_GIT_STATUS=clean
# Detect if we have unchecked in local changes, or if we're not on the master branch (possibly at an
# older revision).
git fetch > /dev/null
# diff-index requires update-index --refresh; see:
# https://stackoverflow.com/questions/36367190/git-diff-files-output-changes-after-git-status/36439778#36439778
if git update-index --refresh > /dev/null ; then
  if ! git diff-index --quiet HEAD -- ; then
    # Repository is dirty due to modified files.
    STABLE_GIT_STATUS=dirty
  elif ! git merge-base --is-ancestor HEAD origin/master ; then
    # Repository is dirty because we're not on the master branch (possibly an older revision).
    STABLE_GIT_STATUS=dirty
  fi
else
  # Repository is dirty due to checked out files.
  STABLE_GIT_STATUS=dirty
fi
echo "STABLE_GIT_STATUS $STABLE_GIT_STATUS"

# If the format of this ever changes then please also update k8s_checker/main.go.
echo "STABLE_DOCKER_TAG ${BUILD_DATETIME}-${USER}-${STABLE_GIT_REVISION:0:7}-${STABLE_GIT_STATUS}"
