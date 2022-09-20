#!/bin/bash
# Update the Kubernetes configurations to use the newly-built images.
set -ex

_WORKSPACE_DIR=$1
_GIT_EMAIL=$2
_GIT_USER=$3

pushd ${_WORKSPACE_DIR}/k8s-config
git checkout -b update -t origin/main

updated_images=""
for tag_file in $(ls ${_WORKSPACE_DIR}/*.tag); do
  image_tag=$(cat $tag_file)
  image_path=$(echo $image_tag | cut -d: -f1)
  image_name=$(basename $image_path)
  docker pull $image_tag
  echo $(docker inspect --format='{{index .RepoDigests 0}}' $image_tag | cut -d@ -f2)
  image_sha256="$image_path@$(docker inspect --format='{{index .RepoDigests 0}}' $image_tag | cut -d@ -f2)"
  find ./ -type f -exec sed -r -i "s;$image_path@sha256:[a-f0-9]+;$image_sha256;g" {} \;
  updated_images="$updated_images image_name"
done

if [[ "$(git diff --exit-code --quiet; echo $?)" == 1 ]]; then
  git config --global user.email "${_GIT_EMAIL}"
  git config --global user.name "${_GIT_USER}"
  mkdir -p .git/hooks
  curl -Lo .git/hooks/commit-msg https://gerrit-review.googlesource.com/tools/hooks/commit-msg
  chmod +x .git/hooks/commit-msg
  git commit -a -m "Update$updated_images"
  git push origin HEAD:refs/for/main%notify=OWNER_REVIEWERS,l=Auto-Submit+1,r=rubber-stamper@appspot.gserviceaccount.com
fi