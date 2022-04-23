#!/bin/bash

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <path to custom element's directory>"
  echo
  echo "Example: $0 golden/modules/dots-sk"
  echo
  echo "This script uses the \"entr\" command to launch a demo page server via"
  echo "\"bazel run\". When the files in the custom element's directory change, entr will"
  echo "reexecute the Bazel command to rebuild and serve the element's demo page with"
  echo "the latest changes. Users must manually refresh the browser to see the changes."
  echo
  echo "Note: This script MUST be executed from the repository's root directory."
  exit 1
fi

# Remove any trailing slashes that might arise from using Bash's autocompletion
# via the tab key.
DIR=$(echo $1 | sed 's:/*$::')

ls $DIR/* | entr -r bazelisk run --config=remote //$DIR:demo_page_server
