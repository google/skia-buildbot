Cluster Telemetry Production Manual
===================================

General information about the Cluster Telemetry is available in the
[design doc](./DESIGN.md).
The [maintenance doc](./maintenance.md) details how to maintain CT's
different components.


Alerts
======

ctfe_pending_tasks
------------------
CT normally picks up tasks in < 1m. Having any task be pending in the
[queue](https://ct.skia.org/queue/) could mean that the CT poller is down
(see below) or that something is wrong with the CT framework possibly related
to a recent push.

ct_poller_health_check
----------------------
The CT poller health check is failing. The poller's error logs are
[here](https://uberchromegw.corp.google.com/i/skia-ct-master/poller.ERROR?page_y=end).
The poller runs on the CT master in the Chrome Golo. See the instructions
[here](https://skia.googlesource.com/buildbot/+/master/ct/maintenance.md#Access-to-Golo)
for how to access the master.

