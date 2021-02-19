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

### regression_detection_slow

The perf instance has not detected any regressions in an hour, which is unlikely
because of the large amount of traces these instances ingest.

Check that data is arriving to the instances that do event driven regression:

https://thanos-query.skia.org/graph?g0.range_input=6h&g0.max_source_resolution=0s&g0.expr=rate(ack%5B30m%5D)&g0.tab=0

Check that PubSub messages are being processed: http://go/android-perf-ingest-stall

And determine when regression detection stopped:

https://thanos-query.skia.org/graph?g0.range_input=1d&g0.max_source_resolution=0s&g0.expr=sum(rate(perf_regression_store_found%7Bapp%3D~%22perf-clustering-android%7Cskiaperf%7Cskiaperf-android-x%22%7D%5B30m%5D))%20by%20(app)&g0.tab=0
