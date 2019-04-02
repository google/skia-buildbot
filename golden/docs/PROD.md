Gold Production Manual (WIP)
===============================

This information is in the process off being filled out - consider it incomplete
or misleading at best for now.

First make sure you are familiar with the design of gold by reading the
[DESIGN](./DESIGN.md) doc.

General Metrics
===============
<https://mon.skia.org/dashboard/db/gold-panel>
TODO
 - What are some key things to look for
 - What are the units on the Load metric? Are there high level lines
 - Can we get these dashboards from prom2 (skia-public) data?

General Logs
============

TODO - where are they?  link to console.cloud.google

Alerts
======

Items below here should include target links from alerts.

GoldIngestionStalled
--------------------
Gold ingestion hasn't completed in at least 10 minutes. Normally, it should
complete every minute or so (90% at 3 minutes) for gold, up to 5 minutes
for other instances (Pdfium, Chrome VR)

TODO: what else should be looked at when diagnosing this?

Key metrics: since-last-run

TODO: common causes and mitigations to try


GoldIgnoreMonitoring
--------------------
TODO: What is this measuring? How are ignore rules ingested?

TODO: what else should be looked at when diagnosing this?

Key metrics: gold_expired_ignore_rules_monitoring

TODO: common causes and mitigations to try


GoldIngestionErrorRate
----------------------
The recent rate of errors for ingestion is high, it is typically well below 0.1.
See the error logs for the given instance for more.

TODO Is the link in alert.rules right?


GoldDiffServerErrorRate
----------------------
The recent rate of errors for the diff server is high, it is typically well
below 0.1.
See the error logs for the given instance for more.

TODO Is the link in alert.rules right?


GoldErrorRate
----------------------
The recent rate of errors for ??? (the web server?) is high, it is
typically well below 0.1.
See the error logs for the given instance for more.

TODO Is the link in alert.rules right?
