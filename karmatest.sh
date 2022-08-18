#!/bin/bash

if [[ $# -ne 1 ]]; then
  echo "Launches a Karma test in interactive mode for in-browser debugging. It watches for file"
  echo "changes in the test's directory and relaunches the Karma test runner when any such files"
  echo "change."
  echo
  echo "Usage: $0 <path to custom element's directory>"
  echo
  echo "As an example, the following two commands run the same Karma test:"
  echo
  echo "    $0 golden/modules/digest-details-sk"
  echo "    bazelisk run --config=remote //golden/modules/digest-details-sk:digest-details-sk_test"
  echo
  echo "This script uses the \"entr\" command to launch a Karma test in interactive mode via"
  echo "\"bazelisk run\". When the files in the custom element's directory change, entr will"
  echo "reexecute the bazelisk command to rebuild and run the element's Karma test in interactive"
  echo "mode with the latest changes. Users must manually refresh the browser to see the changes."
  echo
  echo "Note: This script MUST be executed from the repository's root directory."
  echo "Note: This script only works if the Bazel label of the test is of the form"
  echo "      //path/to/<MODULE>:<MODULE>_test."
  exit 1
fi

# Remove any trailing slashes that might arise from using Bash's autocompletion
# via the tab key, e.g. "path/to/foo/" becomes "path/to/foo".
DIR=$(echo $1 | sed 's:/*$::')

# Extract the Bazel target name
TARGET=$(basename $DIR)_test

ls $DIR/* | entr -r bazelisk run --config=remote //$DIR:$TARGET

# For some reason entr leaves the terminal in a corrupted state after being killed with Ctrl+C. The
# following commands restore the terminal to a workable state.
reset
clear
