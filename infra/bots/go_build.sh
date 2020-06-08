#!/bin/bash

set -e -x

IMAGE="${1}"
OUTPUT_PATH="$(realpath ${2})"

TMP="$(mktemp -d)"

cat << EOF > ${TMP}/build.sh
set -e -x
go build -o /out ./...
for pkg in $(go list -f "{{if .TestGoFiles}}{{.ImportPath}}{{end}}" ./...); do
  mkdir -p ./output/test/$(dirname $pkg)
  go test -vet=off -c -o ./output/test/${pkg}.test ./${pkg#go.skia.org/infra/}
done
EOF
chmod a+x ${TMP}/build.sh

for PLATFORM in ${@:3}; do
  PLATFORM_OUT="${OUTPUT_PATH}/${PLATFORM}"
  mkdir -p "${PLATFORM_OUT}"
  GOOS="$(echo $PLATFORM | cut -d "-" -f 1)"
  GOARCH="$(echo $PLATFORM | cut -d "-" -f 2)"
  docker run
    --env GOOS="${GOOS}" \
    --env GOARCH="${GOARCH}" \
    --mount type=bind,src=$(pwd),dst=/repo \
    --mount type=bind,src=${PLATFORM_OUT},dst=/out \
    --mount type=bind,src=${TMP},dst=/build \
    --workdir /repo \
    "${IMAGE}" \
    /build/build.sh
done

rm -rf "${TMP}"