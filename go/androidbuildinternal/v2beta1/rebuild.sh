#!/bin/bash

# Script to rebuild the androidbuildinternal.go file.

# Get an updated generator. Pin to v0.10.0 because https://github.com/googleapis/google-api-go-client/issues/416.
go get -u google.golang.org/api/google-api-go-generator@v0.10.0

# Retrieve the discovery document for the API.
wget https://www.googleapis.com/discovery/v1/apis/androidbuildinternal/v2beta1/rest

# Generate the Go file.
#
# Note that we specify our own copy of the gensupport library because we are not
# the target audience:
# https://github.com/googleapis/google-api-go-client/issues/416
#
# If this library breaks in the future first try updating our copy of gensupport
# from
# https://github.com/googleapis/google-api-go-client/tree/master/internal/gensupport.
google-api-go-generator \
  -api_json_file=rest \
  -api_pkg_base="go.skia.org/infra/go" \
  -gensupport_pkg="github.com/skia-dev/google-api-go-client/gensupport" \
  -output="androidbuildinternal.go"



# Clean up the generated file.
goimports -w .

# Cleanup.
rm rest
