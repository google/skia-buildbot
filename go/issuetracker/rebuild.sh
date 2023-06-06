#!/bin/bash
# Script to rebuild the issuetracker.go file.

if ! command -v google-api-go-generator &> /dev/null
then
    echo "google-api-go-generator could not be found"
    echo ""
    echo "Run the following command outside of the infra directory:"
    echo ""
    echo "    go install google.golang.org/api/google-api-go-generator@v0.10.0"
    exit
fi

if [[ -z "${APIKEY}" ]]; then
  echo "The APIKEY env variable must be defined."
  echo "The API Key to use is here:"
  echo "  go/skia-issue-tracker-api-key"
  exit
fi

# If this fails make sure you are running as a service account that has access to the API.
#
# See: # go/skia-issue-tracker-api-creds
go run ./getdiscovery.go --apikey=${APIKEY} > rest.json

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
  -api_json_file=rest.json \
  -api_pkg_base="go.skia.org/infra/go" \
  -gensupport_pkg="github.com/skia-dev/google-api-go-client/gensupport" \
  -output="v1/issuetracker.go"

# Clean up the generated file.
goimports -w .

# Cleanup.
rm rest.json
