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

https://prom2.skia.org/graph?g0.range_input=6h&g0.max_source_resolution=0s&g0.expr=rate(ack%5B30m%5D)&g0.tab=0

Check that PubSub messages are being processed:
http://go/android-perf-ingest-stall

And determine when regression detection stopped:

https://prom2.skia.org/graph?g0.range_input=1d&g0.max_source_resolution=0s&g0.expr=sum(rate(perf_regression_store_found%7Bapp%3D~%22perf-clustering-android%7Cskiaperf%7Cskiaperf-android-x%22%7D%5B30m%5D))%20by%20(app)&g0.tab=0

## issue_tracker_rate_high

The number of issues being created in the issue tracker is too high.

To quickly stop Perf from creating more issues set the `--noemail` flag on the
offending instance.

## querying_wedged

This alert is raised when the number of long running queries remains high. It
might just be high traffic, but if `perf_progress_tracker_num_entries_in_cache`
looks constant over time then the querying might be wedged. Check for Go routine
leaks or contents that time out in the logs.

## too_much_data

There are times when a process may inject too much data into Perf.

These may be useful queries to run against the database to find where the data
is coming from:

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

## perf_smoketest_failing

This alert fires when `perf_smoketest_success` stays at 0 for more than 1 hour
for either `chrome-internal-autopush` or `v8-internal-autopush`.

Check which individual smoke check failed by querying
`perf_smoketest_check_success{instance,check}` or inspecting the latest verdict
in GCS and the pod logs:

```
gcloud storage cat gs://skia-perf-smoketest/perfserver/chrome-internal-autopush.json
gcloud storage cat gs://skia-perf-smoketest/perfserver/v8-internal-autopush.json
kubectl --context=gke_skia-infra-corp_us-central1-a_skia-infra-corp -n perf logs deployment/perf-smoketest --tail=100
```

If `historical_query` returns an empty traceset because the pinned benchmark or
bot stopped reporting data, update the target query or 24-hour window in
`skia-infra-corp/perf-smoketest.yaml` in `k8s-config`.

## perf_smoketest_stale

This alert fires when `perf_smoketest_last_run_timestamp` is more than 15
minutes old. Check whether the `perf-smoketest` pod is crash-looping, pending,
or failing to fetch `pipeline/stages.json` from Gitiles or write verdicts to
`gs://skia-perf-smoketest`:

```
kubectl --context=gke_skia-infra-corp_us-central1-a_skia-infra-corp -n perf get pods -l app=perf-smoketest
kubectl --context=gke_skia-infra-corp_us-central1-a_skia-infra-corp -n perf describe pod -l app=perf-smoketest
```

## Smoke test gate and release runbook

The Louhi flow "Promote Perfserver Prod" runs weekly on Tuesday at 10:00 PT (and
can be triggered manually at any time). Before `stagemanager` promotes `latest`
to `prod`, `perf-smoketest gate` verifies that both `chrome-internal-autopush`
and `v8-internal-autopush` have a `pass` verdict for the `latest` `perfserver`
git hash with `consecutive_passes >= 3` and `finished_at` within the last 15
minutes.

### Reading a BLOCK table and re-running

When the gate blocks promotion, the GCB step exits with status 1 and prints a
table listing every failed condition per instance (`status`, `app_version`
mismatch, short streak, or stale verdict age). Fix the underlying autopush issue
or wait for 3 consecutive passing runs (~9 minutes after rollout), then click
Run on the "Promote Perfserver Prod" flow in Louhi.

### Pre-promotion check for grouping_workflow

Smoke tests check only `perfserver`. Before voting `Code-Review +1` on the
generated promotion CL, verify that `grouping_workflow:latest` started and is
polling in `skia-infra-public-dev`:

1. Run:
   ```
   kubectl --context=gke_skia-infra-public-dev_us-central1-a_skia-infra-public-dev -n perf get pods -l app=perf-grouping-worker -o jsonpath='{range .items[*]}{.metadata.name}{"  "}{.status.phase}{"  "}{.status.containerStatuses[0].restartCount}{"  "}{.status.containerStatuses[0].imageID}{"\n"}{end}'
   ```
2. Confirm every pod runs the `latest` `grouping_workflow` digest from
   `pipeline/stages.json`, is `Running`, and is not restarting.
3. Confirm live pollers at
   https://temporal-ui-dev.corp.goog/namespaces/perf-internal/task-queues/perf.perf-chrome-internal.grouping.

### Emergency gate bypass and rollback

To promote during an autopush infrastructure outage unrelated to the build,
trigger "Promote Perfserver Prod" manually in Louhi with `_SKIP_GATE=true`. The
CL description records that the gate was skipped along with the Louhi execution
ID.

To roll back a bad production push, run the "Rollback Perfserver Prod" flow in
Louhi, which restores `prod` from `previous_prod`.

### Updating pinned gate and cd digests in skia-infra

`skia-infra/.louhi/stage-types/promote-perfserver-prod.yaml` pins
`gcr.io/skia-public/perf_smoketest` and `gcr.io/skia-public/cd` by digest. After
modifying `perf/go/smoketest` or `cd/`, rebuild the image in Louhi and update
the digest in `promote-perfserver-prod.yaml`.
