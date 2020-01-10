#!/bin/bash

set -e

# Build the list of all recursive dependent packages, excluding the ones we
# know are going to licensed correctly.
DEPS=`go-licenses csv go.skia.org/infra/... | \
  cut -d, -f2 --complement | \
  sort | \
  (echo -e "Package,License\n," && cat) | \
  sed -e 's/^/| /' -e 's/,/,| /g' -e 's/$/,|/' | \
  column -t -s, | \
  sed -e '2s/ /-/g' | \
  (echo -e "LICENSES\n========\n\nThe following licenses are used in dependent packages.\n" && cat)
`

if [ "$1" = "regenerate" ]; then
  echo "$DEPS" > LICENSES.md
elif [ "$1" = "" ]; then
  diff -s <(echo "$DEPS") LICENSES.md
  if [ $? != 0 ]; then
    echo "Check failed. See licenses/README.md on how to fix the failures."
    exit 1
  fi
else
  echo "Run with either no arguments to check the list of dependent packages, or with an argument of 'regenerate' to regenerate the list of dependent packages."
  exit 1
fi

go-licenses check go.skia.org/infra/...
