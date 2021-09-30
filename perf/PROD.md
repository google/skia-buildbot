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

Check that PubSub messages are being processed:
http://go/android-perf-ingest-stall

And determine when regression detection stopped:

https://thanos-query.skia.org/graph?g0.range_input=1d&g0.max_source_resolution=0s&g0.expr=sum(rate(perf_regression_store_found%7Bapp%3D~%22perf-clustering-android%7Cskiaperf%7Cskiaperf-android-x%22%7D%5B30m%5D))%20by%20(app)&g0.tab=0

## too_much_data

There are times when a process may inject too much data into Perf.

These may be useful queries to run against the database to find where the data
it coming from:

# Sample a subset of param_values.

```
SELECT param_value FROM paramsets WHERE tile_number=265 AND param_key='sub_result' ORDER BY random() LIMIT 100;
```

You can look at the (Perf dashboard)[https://grafana2.skia.org/d/VNdBF9Ciz/perf]
to see relevant tile numbers.

Once you find the paramsets that are affected you can count how many new
paramset values are being added, in this case we have found from the above
sampling query that the param_value always begins with `showmap_granular`, so we
construct a query that limits us to those param_values.

```
SELECT
    COUNT(param_value)
FROM
    paramsets
AS OF SYSTEM TIME '-5s'
WHERE
  tile_number=283
  AND param_key='sub_result'
  AND param_value>'showmap_granular'
  AND param_value<'showmap_granulas';
```

For example:

```
root@perf-cockroachdb-public:26257/android> SELECT
  COUNT(param_value)
FROM
  paramsets
WHERE
  tile_number=282
  AND param_key='sub_result'
  AND param_value>'showmap_granular'
  AND param_value<'showmap_granulas';

   count
+---------+
  9687198
(1 row)

Time: 8.331250012s
```

And then you can use the same query to remove all the matching paramsets:

```
DELETE
FROM
  paramsets
WHERE
  tile_number=282
  AND param_key='sub_result'
  AND param_value>'showmap_granular'
  AND param_value<'showmap_granulas';
```

Make sure to remove the erroneous params from all the tiles where they appear.

You may encounter contention that will slow the deletes, particularly if there
are any rows to delete. It will help to temporarily scale the number of
clusterers down to zero:

```
kubectl scale --replicas=0 deployment/perf-clustering-android
```

Don't forget to scale them back up!

Also deleting in batches will also speed things up, trying different values for
the LIMIT:

```
DELETE
FROM
    paramsets
WHERE
    tile_number=282 AND
    param_key='sub_result' AND
    param_value>'showmap_granular' AND
    param_value<'showmap_granulas' LIMIT 100000
```

The bash file `//perf/migrations/batch-delete.sh` does batches of deletes using
`//perf/migrations/batch-delete.sql` as the SQL to run. Modify that file to
control which params to delete.
