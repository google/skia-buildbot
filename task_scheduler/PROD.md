Task Scheduler Production Manual
================================

General information about the Task Scheduler is available in the
[README](./README.md).


Alerts
======

scheduling_failed
----------------------

The Task Scheduler has failed to schedule for some time. You should check the
logs to try to diagnose what's failing. It's also possible that the scheduler
has slowed down substantially and simply hasn't actually completed a scheduling
loop in the required time period. That needs to be addressed with additional
optimization.


http_latency
------------

The server is taking too long to respond. Look at the logs to determine why it
is slow.


error_rate
----------

The server is logging errors at a higher-than-normal rate. This warrants
investigation in the logs.

