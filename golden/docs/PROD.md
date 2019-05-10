Gold Production Manual
======================

First make sure you are familiar with the design of gold by reading the
[architectural overview](https://goto.google.com/life-of-a-gold-image) doc.

General Metrics
===============
The following dashboard is for the legacy, non-k8s instances:
<https://mon.skia.org/dashboard/db/gold-panel>.

The following dashboard is for the skia-public instances:
<https://grafana2.skia.org/d/m8kl1amWk/gold-panel-public>.

The following dashboard is for the skia-corp instances:
<https://skia-mon.corp.goog/d/m8kl1amWk/gold-panel-corp>

Some things to look for:

 - Do goroutines or memory increase continuously (e.g leaks)?
 - How fresh is the tile data? (this could indicate something is stuck).
 - How is ingestion liveness?  Anything stuck?

General Logs
============

Logs for legacy, non-k8s instances are linked to by push.skia.org, e.g.:
<https://console.cloud.google.com/logs/viewer?project=google.com:skia-buildbots&resource=logging_log%2Fname%2Fskia-gold-prod&logName=projects%2Fgoogle.com:skia-buildbots%2Flogs%2Fskiacorrectness-prod>

Logs for Gold instances in skia-public/skia-corp are in the usual
GKE container grouping, for example:
<https://console.cloud.google.com/logs/viewer?project=skia-public&resource=container&logName=projects%2Fskia-public%2Flogs%2Fgold-flutter-skiacorrectness>

Alerts
======

Items below here should include target links from alerts.

GoldIngestionStalled
--------------------
Gold ingestion hasn't completed in at least 10 minutes. Normally, it should
complete every minute or so (90% at 3 minutes) for gold, up to 5 minutes
for other instances (Pdfium, Chrome VR)

Ingestion depends on the tile being served, so check the logs for which commits
were produced in the last tile (search logs for "last commit"). This has
happened before because gitsync stopped, so check that out too.

Key metrics: since_last_run, liveness_last_successful_git_sync_s


GoldIgnoreMonitoring
--------------------
This alert means gold was unable to calculate which ignore rules were expired.
Search the logs for "ignorestore.go" to get a hint as to why.

This has happened before because of manually-edited (and incorrect) Datastore data
so maybe check out the raw data
<https://console.cloud.google.com/datastore/entities;kind=IgnoreRule;ns=gold-flutter/query/kind?project=skia-public>
and
<https://console.cloud.google.com/datastore/entities;kind=HelperRecentKeys;ns=gold-flutter/query/kind?project=skia-public>

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
Gold has been unable to re-process some of the data used to keep the
frontend up to date.

This hasn't happened yet, but likely would indicate a problem with
something in golden/go/status.go

Key metrics: liveness_gold_status_monitoring_s

GoldIngestionErrorRate
----------------------
The recent rate of errors for ingestion is high, it is typically well below 0.1.
See the error logs for the given instance for more.

GoldDiffServerErrorRate
----------------------
The recent rate of errors for the diff server is high, it is typically well
below 0.1.
See the error logs for the given instance for more.

GoldErrorRate
----------------------
The recent rate of errors for the main gold instance is high, it is
typically well below 0.1.
See the error logs for the given instance for more.

