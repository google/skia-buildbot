#!/bin/bash

# Script to rebuild the androidbuildinternal.go file.

# Get an updated generator.
go get -u github.com/google/google-api-go-client/google-api-go-generator

# Retrieve the discovery document for the API.
wget https://www.googleapis.com/discovery/v1/apis/androidbuildinternal/v2beta1/rest

# Generate the Go file.
google-api-go-generator \
  -api_json_file=rest \
  -api_pkg_base="go.skia.org/infra/go" \
  -output="androidbuildinternal.go"

# Clean up the generated file.
goimports -w .

# Cleanup.
rm rest
