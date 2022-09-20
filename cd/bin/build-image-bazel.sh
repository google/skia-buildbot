#!/bin/bash
# Build a Docker image using Bazel.
set -ex

_WORKSPACE_DIR=$1
_CHECKOUT_DIR=$2
_BAZEL_PACKAGE=$3
_BAZEL_TARGET=$4
_IMAGE_PATH=$5

pushd ${_WORKSPACE_DIR}/${_CHECKOUT_DIR}
bazelisk run --config=remote --google_default_credentials //${_BAZEL_PACKAGE}:${_BAZEL_TARGET}
image_tag="${_IMAGE_PATH}:$(USER="louhi" ${_WORKSPACE_DIR}/${_CHECKOUT_DIR}/bash/release_tag.sh)"
echo "$image_tag" > ${_WORKSPACE_DIR}/${_BAZEL_TARGET}.tag
docker tag bazel/${_BAZEL_PACKAGE}:${_BAZEL_TARGET} louhi_ws/$image_tag