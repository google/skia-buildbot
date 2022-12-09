#!/bin/bash
#
# Uploads all the JSON files under ./downloads to the Skia Perf bucket for
# ingestion.

gsutil \
  -m cp -r \
  downloads/ \
  gs://skia-perf/nano-json-v1/$(date -u --date +1hour +%Y/%m/%d/%H)
