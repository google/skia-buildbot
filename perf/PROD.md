# Perf Production Manual

## Alerts

### success_rate_too_low

The rate of successful ingestion is too low. Look for errors in the logs of the
perf-ingest process.

### android_clustering_rate

Android Clustering Rate is too low. Look to see if PubSub events are being sent:
http://go/android-perf-ingest-stall

Also confirm that files are being sent with actual data in them (sometimes they
can be corrupted with a bad config on the sending side). Look in:

    gs://skia-perf/android-master-ingest/tx_log/

### clustering_rate

Perf Clustering Rate is too low. Look to see if PubSub events are being sent:

Also confirm that files are being sent with actual data in them (sometimes they
can be corrupted with a bad config on the sending side). Look in:

    gs://skia-perf/nano-json-v1/
