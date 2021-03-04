Gold Production Manual
======================

First make sure you are familiar with the design of gold by reading the
[architectural overview](https://goto.google.com/life-of-a-gold-image) doc.

Clients can file a bug against Gold at [go/gold-bug](https://goto.google.com/gold-bug).

General Metrics
===============
The following dashboard is for the skia-public instances:
<https://grafana2.skia.org/d/m8kl1amWk/gold-panel-public>.

The following dashboard is for the skia-corp instances:
<https://skia-mon.corp.goog/d/m8kl1amWk/gold-panel-corp>

Some things to look for:

 - Do goroutines or memory increase continuously (e.g leaks)?
 - How fresh is the tile data? (this could indicate something is stuck).
 - How is ingestion liveness?  Anything stuck?

QPS
---
To investigate the load of Gold's RPCs navigate to https://thanos-query.skia.org and
try doing the search:

    rate(gold_rpc_call_counter[1m])

You can use the func timers to even search by package, e.g. finding QPS of all
Firestore related functions:

    rate(timer_func_timer_ns_count{package=~".+fs_.+"}[1m])

If you find something problematic, then `timer_func_timer_ns{appgroup=~"gold.+"}/1000000000` is
how to see how many milliseconds a given timer actually took.

General Logs
============
Logs for Gold instances in skia-public/skia-corp are in the usual
GKE container grouping, for example:
<https://console.cloud.google.com/logs/viewer?project=skia-public&resource=container&logName=projects%2Fskia-public%2Flogs%2Fgold-flutter-frontend>

Opencensus Tracing
==================
We export Open Census tracing to Stackdriver in Google Cloud. These traces are handy for diagnosing
performance. To find the traces, search for them at <http://console.cloud.google.com/traces/list>.

When adding a new application (or retrofitting open census tracing onto an existing app), be sure
to call tracing.Initialize() so they can be reported.

Managing the CockroachDB (SQL) database
=======================================
The most reliable way to have open and use a connection to the SQL database is to create an
ephemeral k8s pod that runs the CockroachDB SQL CLI.
```
kubectl run -it --rm gold-cockroachdb-temp-0 --restart=Never --image=cockroachdb/cockroach:v20.2.0 \
  -- sql --insecure --host gold-cockroachdb:26234 --database [instance_name]
```
The CockroachDB cluster is currently 5 nodes, with a replication factor of 3. By connecting to the
host `gold-cockroachdb:26234`, the ephemeral k8s pod will connect to one of the nodes at random.

The single cluster houses data from all instances, each under their own database. You'll need to
fill in the instance name above. Not sure which instances are there? Omit the --database param
from the above command and then run `SHOW DATABASES;` after connecting.

CockroachDB was setup using [the recommended k8s flow](https://www.cockroachlabs.com/docs/v20.2/orchestrate-cockroachdb-with-kubernetes-insecure#manual).
Some adjustments were made to the default config to allow for more RAM and disk size per node.

Updating the version of CockroachDB
-----------------------------------
The k8s configuration that is used for CockroachDB should be able to be easily upgraded by modifying
the version indicated in the
[.yaml file](https://skia.googlesource.com/k8s-config/+/refs/heads/master/skia-public/gold-cockroach-statefulset.yaml).
Applying this will automatically take one node down at a time and restart it with the new binary.
See [this procedure](https://www.cockroachlabs.com/docs/stable/orchestrate-cockroachdb-with-kubernetes.html#upgrade-the-cluster)
to make sure there aren't any steps needed for minor or major version upgrades.

Debugging and Information Gathering Tips
----------------------------------------

  - `SHOW QUERIES` and `SHOW JOBS` is a way to see the ongoing queries (if things appear jammed up).
  - `SHOW STATISTICS FOR TABLE [foo]` is a handy way to get row counts and distinct value counts.
  - `SHOW RANGES FROM TABLE [foo]` is a good way to see the ranges, how full each is and where the
    keys are partitioned.

Replacing a Node
----------------
Suppose a node is jammed or wedged or otherwise seems to be having issues. One thing to try is to
decommission it, delete it, and recreate it. For this example, let's suppose node 2 is the node
we want to replace.

First, let's decommission node 2, that is, we tell other nodes to stop using node 2.
```
kubectl run -it --rm gold-cockroachdb-temp-0 --restart=Never --image=cockroachdb/cockroach:v20.2.0 \
  -- node decommission 2 --insecure --host gold-cockroachdb:26234
```
This process makes new copies of the ranges that were on node 2 and makes them available on the
other nodes. While this is running, we can delete node 2 and its underlying disk.
```
# Temporarily stop k8s from healing the pod
kubectl delete statefulsets gold-cockroachdb --cascade=false
kubectl delete pod gold-cockroachdb-2
kubectl delete pvc datadir-gold-cockroachdb-2
# Make sure the disk is gone
kubectl get persistentvolumes | grep gold
```
When the node is deleted, we should be able to re-apply the statefulset k8s yaml and the missing
pod will be re-created. It should automatically connect to the cluster. Then, some ranges will
assigned to it. Note that this new node may have a different number according to cockroachDB than
the k8s pod id.

Advice for a staging instance
-----------------------------
If a staging instance is desired, it is recommended to use TSV files and IMPORT to load the
database with a non-trivial amount of data.

Cluster Authentication
----------------------
How does the CockroachDB cluster authenticate to GCS to write backups?

There are two service accounts (one for public, one for corp) that have been loaded
into the cluster, as a cluster setting and are used as the default credentials.
 - `gold-cockroachdb@skia-public.iam.gserviceaccount.com`
 - `gold-cockroachdb@skia-corp.google.com.iam.gserviceaccount.com`

These service accounts have been granted the appropriate read/write credentials for
making backups. That is, the accounts can read/write the appropriate backup bucket **only**.
See the [CockroachDB docs](https://www.cockroachlabs.com/docs/stable/use-cloud-storage-for-bulk-operations.html#considerations)
for more information.

```
# Run this command once after the cluster has been created, using the JSON downloaded
# from the cloud console for the appropriate service account.
# Notice the single quotes. The JSON will be double quoted, so this works out nicely.
set cluster setting cloudstorage.gs.default.key = '{...}'
```

Alerts
======

Items below here should include target links from alerts.

GoldStreamingIngestionStalled
--------------------
Gold has a pubsub subscription for events created in its bucket.
This alert means we haven't successfully ingested a file in over 24 hours.
This could mean that ingestion is throwing errors on every file or
the repo isn't very busy.

This has happened before because gitsync stopped, so check that out too.

Key metrics: liveness_gold_ingestion_s{metric="since_last_successful_streaming_result"},
    liveness_last_successful_git_sync_s


GoldPollingIngestionStalled
--------------------
Gold regularly polls its GCS buckets for any files that were not
successfully ingested via PubSub event when the file was created (aka "streaming").
This alert means it has been at least 10 minutes since this happened;
this should happen every 5 minutes or so, even in not-busy repos.

This has happened before because gitsync stopped, so check that out too.

Key metrics: liveness_gold_ingestion_s{metric="since_last_successful_poll"},
    liveness_last_successful_git_sync_s


GoldIgnoreMonitoring
--------------------
This alert means gold was unable to calculate which ignore rules were expired.
Search the logs for "ignorestore.go" to get a hint as to why.

This has happened before because of manually-edited (and incorrect) Firestore data
so maybe check out the raw data
<https://console.cloud.google.com/firestore/data/gold/skia/ignorestore_rules?project=skia-firestore>

Key metrics: gold_expired_ignore_rules_monitoring

GoldCommitTooOldWallTime
----------------------
Too much time has elapsed since Gold noticed a commit. This occasionally is a false positive
if a commit simply hasn't landed in the repo we are tracking.

In the past, this has indicated git-sync might have had problems, so check out
the logs of the relevant git-sync instance.

Key metrics: gold_last_commit_age_s

GoldCommitTooOldNewerCommit
----------------------
Gold has noticed there is a newer commit available for processing, but hasn't
succeeded on moving forward.

This would usually indicate an issue with Gold itself, so check
the logs of the Gold instance.

Key metrics: gold_last_commit_age_s

GoldStatusStalled
----------------------
The underlying metric here is reset when the frontend status is recomputed. This
normally gets recomputed when the Gold sliding window of N commits (aka "tile")
is updated or when expectations are changed (e.g. something gets triaged).

This could fire because of a problem in golden/go/status.go or computing the current
tile takes longer than the minimum for the alert.

Key metrics: liveness_gold_status_monitoring_s

GoldIngestionErrorRate
----------------------
The recent rate of errors for ingestion is high, it is typically well below 0.1.
See the error logs for the given instance for more.

GoldErrorRate
----------------------
The recent rate of errors for the main gold instance is high, it is
typically well below 0.1.
See the error logs for the given instance for more.

GoldExpectationsStale
----------------------
Currently, our baseline servers use QuerySnapshotIterators when fetching expectations out of
Firestore. Those run on goroutines. This alert will be active if any of those sharded
iterators are down, thus yielding stale results.

To fix, delete one baseliner pod of the affected instance at a time until all of them
have restarted and are healthy.

If this alert fires, it probably means the related logic in fs_expstore needs to be rethought.

GoldNoDataAtHead
----------------
The last 20 commits (100 for Chrome, since their tests are slower) have seen 0 data. This probably
means something is wrong with goldctl or whatever means is getting data into gold.

Check out the bucket for the instance to confirm nothing is being uploaded and the logs
of the ingester if newer stuff is in the bucket, but hasn't been processed already. (If it's
an issue with ingestion, expect other alerts to be firing)

Key metrics: gold_empty_commits_at_head

GoldTooManyCLs
--------------
There are many open CLs that have recently seen data from Gold. Having too many open CLs may cause
a higher load on CodeReviewSystems (e.g. Gerrit, GitHub) than usual, as we scan over all of these
to see if they are still open. Seeing this alert may indicate issues with marking CLs as closed
or some other problem with processing CLs.

Key metrics: gold_num_recent_open_cls

GoldCommentingStalled
---------------------
Gold hasn't been able to go through all the open CLs that have produced data and decide whether
to comment on them or not in a while. The presence of this alert might mean we are seeing errors
 when talking to Firestore or to the Code Review System (CRS). Check the logs on that pod's
frontend server to see what's up.

This might mean we are doing too much and running out of quota to talk to the CRS.  Usually
out of quota messages will be in the error messages or the bodies of the failing requests.

Key metrics: liveness_gold_comment_monitoring_s, gold_num_recent_open_cls

rate(firestore_ops_count{app=~"gold.+"}[10m]) > 100

HighFirestoreUsageBurst or HighFirestoreUsageSustainedGold
----------------------------------------------------------
This type of alert means that Gold is probably using more Firestore quota than expected. In an
extreme case, this can exhaust our project's entire Firestore quota (it's shared, unfortunately)
causing wider outages.

In addition to the advice of identifying QPS above, it can be helpful to identify which collections
are receiving a lot of reads/writes. For this, a query like:

```
    rate(firestore_ops_count{app=~"gold.+"}[10m]) > 100
```

can help identify those and possibly narrow in on the cause. `rate(gold_rpc_call_counter[1m]) > 1`
is also a good query to cross-reference this with.

GoldHeavyTraffic
----------------
Gold is seeing over 50 QPS to a specific RPC. As of writing, there are only two RPCs that
are not throttled from anonymous traffic, so it is likely one of these. See <https://skbug.com/9476>
and <https://skbug.com/10768> for more context on these.

This is potentially problematic in that the excess load could be causing Gold to act slowly
or even affect other tenants of the k8s pod. The cause of this load should be identified.

Backups
=======
We use CockroachDB's [automated backup system](https://www.cockroachlabs.com/docs/stable/create-schedule-for-backup.html)
to automatically backup tables. These scheduled activities are stored cluster-wide and can be seen
by running `SHOW SCHEDULES;`

Restoring from automatic backups
--------------------------------
The following shows an example of restoring two tables from backups.
```
RESTORE skiainfra.commits, skiainfra.expectationdeltas
FROM 'gs://skia-gold-sql-backups/skiainfra/daily/2021/01/12-000000.00'
WITH into_db = 'skiainfra';
```

The location on the second line leads to the directory with the backup files. The third line is
optional and can be modified to restore into a different database than the backup originated (maybe
to set up a staging instance or otherwise verify the backup data).

See the [CockroachDB docs](https://www.cockroachlabs.com/docs/v20.2/restore) for more details.

The internal cluster is backed up to `gs://skia-gold-sql-corp-backups`