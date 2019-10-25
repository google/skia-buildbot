#!/bin/bash

# Script to rebuild the monorail.go file.

# Get an updated generator. Pin to v0.10.0 because https://github.com/googleapis/google-api-go-client/issues/416.
go get -u google.golang.org/api/google-api-go-generator@v0.10.0

# Retrieve the discovery document for the API.
wget https://monorail-prod.appspot.com/_ah/api/discovery/v1/apis/monorail/v1/rest

# Generate the Go file.
google-api-go-generator \
  -api_json_file=rest \
  -api_pkg_base="go.skia.org/infra/go" \
  -output="monorail.go"

# Clean up the generated file.
goimports -w .

# Cleanup.
rm rest
