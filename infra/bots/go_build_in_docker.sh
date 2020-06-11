#!/bin/bash
set -e -x

go build -o /out ./...
for pkg in $(go list -f "{{if .TestGoFiles}}{{.ImportPath}}{{end}}" ./...); do
  mkdir -p ./output/test/$(dirname $pkg)
  go test -vet=off -c -o ./output/test/${pkg}.test ./${pkg#go.skia.org/infra/}
done