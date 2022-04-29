#!/bin/bash

# Install the necessary dependencies
set -e -x

go get -u github.com/kisielk/errcheck \
          golang.org/x/tools/cmd/goimports

go get -u github.com/golang/protobuf/protoc-gen-go \
          golang.org/x/tools/cmd/stringer \
          github.com/twitchtv/twirp/protoc-gen-twirp \
          github.com/skia-dev/protoc-gen-twirp_typescript

