#!/bin/bash

# Use this file to force re-ingest data that has arrived from Android.
#
# This script reuses the transmission logs, which records every file POST'd to
# https://android-metric-ingest.skia.org/upload, and uses those logs to simulate
# those files as if they were newly POST'd.

# Alter the date argument to the gsutil ls command to determine which part of
# the log to re-transmit.

gsutil ls "gs://skia-perf/android-master-ingest/tx_log/2021/02/17/**" | xargs -L 1 -P 10 -I {} sh -c "echo {}; gsutil cp {} - | curl --include  --data @- https://android-metric-ingest.skia.org/upload"