#!/bin/bash

set -e

path="${1}"
out="${2}"

if [[ -z "${path}" ]] || [[ -z "${out}" ]]; then
  >&2 echo "Usage ${0} <build path> <out path>"
  exit 1
fi

program="$(basename $path)"

valid_os=(linux darwin windows)

for os in ${valid_os[@]}; do
  valid_arch=(amd64 386)
  if [ "linux" = "${os}" ]; then
    valid_arch=(arm arm64 amd64 386)
  fi
  for arch in ${valid_arch[@]}; do
    arm=""
    if [ "arm" = "${arch}" ]; then
      arm="7"
    fi
    ext=""
    if [ "windows" = "${os}" ]; then
      ext=".exe"
    fi

    # Perform the build.
    export GOOS="${os}"
    export GOARCH="${arch}"
    export GOARM="${arm}"
    dest_dir="${out}/${os}_${arch}"
    echo "mkdir -p ${dest_dir}"
    mkdir -p "${dest_dir}"
    dest_file="${dest_dir}/${program}${ext}"
    go build -o "${dest_file}" -v "${path}"
    file "${dest_file}"
  done
done
