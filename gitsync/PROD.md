GitSync Production Manual
=========================

General Metrics
===============
The following dashboard is for the skia-public instances:
<https://grafana2.skia.org/d/Onp7_5FWk/gitsync>

The following dashboard is for the skia-corp instances:
<https://skia-mon.corp.goog/d/Wi0Yu5FZk/gitsync>

Some things to look for:

 - Do goroutines or memory increase continuously (e.g leaks)?
 - Have any repos taken more than a few seconds to sync?
 - Is there an elevated error rate?

General Logs
============
Logs for GitSync instances in skia-public/skia-corp are in the usual
GKE container grouping, for example:
<https://console.cloud.google.com/logs/viewer?project=skia-public&resource=container&logName=projects%2Fskia-public%2Flogs%2Fgitsync2>

Alerts
======

Items below here should include target links from alerts.

GitSyncStalled
--------------
This alert means we haven't successfully synced a repo in over 5 minutes. This
could be due to failure to communicate with the Gitiles server, or because of a
problem with GitSync itself. Check the logs for details.

Key metrics: liveness_last_successful_git_sync_s


GitSyncErrorRate
----------------
The log error rate is elevated. There are a number of possible causes; check the
logs and verify that things are working as expected.

Key metrics: rate(num_log_lines{level="ERROR",app=~"gitsync.\*"}[30m])
