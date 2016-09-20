#!/bin/bash
#
# Uploads all the JSON files under ./downloads to the Skia Perf bucket for
# ingestion.

~/projects/depot_tools/gsutil.py -m cp -r downloads/ gs://chromium-skia-gm/nano-json-v1/$(date +%Y/%m/%d/%H)
