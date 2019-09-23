# Perf Production Manual

## Alerts

### nack

The rate of Nacks for incoming files to ingest is too high. Look for errors
in the logs for perf-ingest processes.

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
