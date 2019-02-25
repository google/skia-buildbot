#!/bin/bash

go list -f '{{join .Deps "\n"}}' ../... | sort | uniq | grep "[a-zA-Z0-9]\+\." | \
  egrep -v \(cloud.google.com\|go.chromium.org\|go.chromium.org\|golang.org\|google.golang.org\|go.opencensus.io\|go.skia.org\|k8s.io\) > all_deps.txt
