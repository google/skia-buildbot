#!/bin/bash

git fetch
# diff-index requires update-index --refresh; see:
# https://stackoverflow.com/questions/36367190/git-diff-files-output-changes-after-git-status/36439778#36439778
git update-index --refresh > /dev/null
if ! git diff-index --quiet HEAD -- ; then
    echo "dirty"
elif ! git merge-base --is-ancestor HEAD origin/main ; then
      echo "dirty"
else
    echo "clean"
fi