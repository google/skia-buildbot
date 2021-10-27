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

GoldIgnoreMonitoring
--------------------
This alert means gold was unable to calculate which ignore rules were expired.
Search the logs for "ignorestore.go" to get a hint as to why.

This has happened before because of manually-edited (and incorrect) Firestore data
so maybe check out the raw data
<https://console.cloud.google.com/firestore/data/gold/skia/ignorestore_rules?project=skia-firestore>

Key metrics: gold_expired_ignore_rules_monitoring

GoldIngestionErrorRate
----------------------
The recent rate of errors for ingestion is high, it is typically well below 0.1.
See the error logs for the given instance for more.

A common scenario that triggers this is cherry-picks onto non-primary branches that upload to Gold.
When Gold ingests a file and it can't tie it to the primary branch, that is an error.

GoldErrorRate
----------------------
The recent rate of errors for the main gold instance is high, it is
typically well below 0.1.
See the error logs for the given instance for more.

GoldCommentingStalled
---------------------
Gold hasn't been able to go through all the open CLs that have produced data and decide whether
to comment on them or not in a while. The presence of this alert might mean we are seeing errors
 when talking to Firestore or to the Code Review System (CRS). Check the logs on that pod's
frontend server to see what's up.

This might mean we are doing too much and running out of quota to talk to the CRS.  Usually
out of quota messages will be in the error messages or the bodies of the failing requests.

Key metrics: liveness_periodic_tasks_s{task="commentOnCLs"}

GoldDigestSyncingStalled
---------------------
Gold hasn't been able to sync the known digests to the GCS. See KnownHashesGCSPath in the config
for the actual file path.

If the GCS file is stale, it means our tests might be doing a bit of extra work encoding, writing,
and uploading images that are already there. Check the logs of periodictasks to see what is
going wrong.

Key metrics: liveness_periodic_tasks_s{task="syncKnownDigests"}

GoldHeavyTraffic
----------------
Gold is seeing over 50 QPS to a specific RPC. As of writing, there are only two RPCs that
are not throttled from anonymous traffic, so it is likely one of these. See <https://skbug.com/9476>
and <https://skbug.com/10768> for more context on these.

This is potentially problematic in that the excess load could be causing Gold to act slowly
or even affect other tenants of the k8s pod. The cause of this load should be identified.

GoldDiffCalcBehind
--------------------
Gold has fallen behind calculating diffs.

It is best to look at the logs for the app (e.g. gold-skia-diffcalculator) to see any
error messages. It might mean that there is too much work and we need to scale up the
number of workers.

`kubectl scale deployment/gold-FOO-diffcalculator --replicas 8`

Key metrics: diffcalculator_workqueuesize

GoldDiffCalcStale
--------------------
Gold's diffs are getting too old. If this continues, CLs and recent images on the primary
branch will not have any diff metrics to search by.

It is best to look at the logs for the app (e.g. gold-skia-diffcalculator) to see any
error messages.

Key metrics: diffcalculator_workqueuefreshness

GoldIngestionFailures
---------------------
Gold is failing to process a high percentage of files. This could be the clients fault (e.g.
sending us bad data via goldctl), something could be misconfigured in Gold, or there could be
an outage on a third party service (e.g. BuildBucket).

It is best to look at the logs for the app (e.g. gold-skia-ingestion) to see the error messages.

The alert is set up to look at the percentage of failures over the last 10 minutes.

Key metrics: gold_ingestion_failure, gold_ingestion_success

GoldPollingIngestionStalled
---------------------------
As a backup to the Pub/Sub polling, Gold will scan all the ingested files produced over the last
two hours. It does so every hour, so files shouldn't be missed. If this alert is firing, something
is causing the polling to take too long (or the backup has stopped).

A cause of this in the past was a lack of Pub/Sub events being fired when objects landed in the
bucket for Tryjob data. As a result, the only way tryjobs were being ingested was via the backup.
The backup took too long to do this and the alert fired.

Check the ingestion logs to see if there are a lot of files that are not being ignored on the
backup polling (and thus, are actually being ingested). Use //golden/cmd/pubsubtool to list or
create new bucket subscriptions as necessary. (See go/setup-gold-instance for details).

Key metrics: liveness_gold_ingestion_s{metric="since_last_successful_poll"}

GoldStreamingIngestionStalled
-----------------------------
Gold hasn't ingested a file via Pub/Sub in a while. This commonly happens when there is no data
(e.g. a weekend).

Key metrics: liveness_gold_ingestion_s{metric="since_last_successful_streaming_result"}

GoldSQLBackupError
------------------
The automatic SQL backups (see below) are not running as expected for the given instance. Check the
logs of the periodictasks service for the given instance, or run the `SHOW SCHEDULES` command
outlined below for the exact details of the failure.

In the past, this has failed due to auth issues (should be alleviated due to using Workload
Identity) or tables missing. If the latter, we can recreate the schedules using `sqlinit`
(see below).

Once all three schedules (daily, weekly, monthly) are working, the metric will go back to 0
(aka, error-free).

Key metrics: periodictasks_backup_error

Backups
=======
We use CockroachDB's [automated backup system](https://www.cockroachlabs.com/docs/stable/create-schedule-for-backup.html)
to automatically backup tables. These scheduled activities are stored cluster-wide and can be seen
by running `SHOW SCHEDULES;`.

A quick summary can be listed with `SELECT id, label, state FROM [SHOW SCHEDULES] ORDER BY 2;`
In that view, "state" would contain any errors of the last cycle.

The backup schedule needs to be re-created if we ever add/remove/rename a table. This can be
achieved by running `go run ./cmd/sqlinit --db_name <instance>` for all instances.

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