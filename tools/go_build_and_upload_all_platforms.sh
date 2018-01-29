#!/bin/bash

# Build the given path for all platforms and upload the binaries to the skia-public-binaries bucket.

set -e

path="${1}"

if [[ -z "${path}" ]]; then
  >&2 echo "Usage ${0} <build path>"
  exit 1
fi

name="$(basename ${path})"
workdir="/tmp/out/${name}"
rm -rf "${workdir}"
mkdir -p "${workdir}"

$(dirname ${0})/go_build_all_platforms.sh "${path}" "${workdir}"

gsutil cp -r ${workdir} gs://skia-public-binaries

rm -rf ${workdir}
