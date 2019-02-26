#!/bin/bash

DEPS=`go list -f '{{join .Deps "\n"}}' ../... | sort | uniq | grep "[a-zA-Z0-9]\+\." | \
  egrep -v \(cloud.google.com\|github.com/prometheus\|go.chromium.org\|go.chromium.org\|golang.org\|google.golang.org\|go.opencensus.io\|go.skia.org\|k8s.io\)`

if [ "$1" = "regenerate" ]; then
  echo "$DEPS" > all_deps.txt
elif [ "$1" = "" ]; then
  diff -s <(echo "$DEPS") all_deps.txt
else
  echo "Run with either no arguments to check the list of dependent packages, or with an argument of 'regenerate' to regenerate the list of dependent packages."
  exit 1
fi

