Datahopper Production Manual
============================

Alerts
======

job_metrics
-----------

The [job
metrics](https://skia.googlesource.com/buildbot/+/master/datahopper/go/datahopper/jobs.go)
goroutine has not successfully updated its job cache for some time.

If there are Task Scheduler alerts, resolve those first.

Otherwise, you should check the
[logs](https://console.cloud.google.com/logs/viewer?project=google.com:skia-buildbots&minLogLevel=500&expandAll=false&resource=logging_log%2Fname%2Fskia-datahopper2&logName=projects%2Fgoogle.com:skia-buildbots%2Flogs%2Fdatahopper)
to try to diagnose what's failing.


bot_coverage_metrics
--------------------

The [bot coverage
metrics](https://skia.googlesource.com/buildbot/+/master/datahopper/go/bot_metrics/bot_metrics.go)
goroutine has not successfully completed a cycle for some time. You should
check the
[logs](https://console.cloud.google.com/logs/viewer?project=google.com:skia-buildbots&minLogLevel=500&expandAll=false&resource=logging_log%2Fname%2Fskia-datahopper2&logName=projects%2Fgoogle.com:skia-buildbots%2Flogs%2Fdatahopper)
to try to diagnose what's failing.


swarming_task_metrics
--------------------

The [Swarming task
metrics](https://skia.googlesource.com/buildbot/+/master/datahopper/go/swarming_metrics/tasks.go)
goroutine has not successfully queried for Swarming tasks for some time. You should
check the
[logs](https://console.cloud.google.com/logs/viewer?project=google.com:skia-buildbots&minLogLevel=500&expandAll=false&resource=logging_log%2Fname%2Fskia-datahopper2&logName=projects%2Fgoogle.com:skia-buildbots%2Flogs%2Fdatahopper)
to try to diagnose what's failing.


event_metrics
-------------

The [event
metrics](https://skia.googlesource.com/buildbot/+/master/go/metrics2/events/events.go)
goroutine has not successfully updated metrics based on event data for some
time. You should check the logs to try to diagnose what's failing. Double-check
the instance name to verify which log stream to investigate.


swarming_bot_metrics
--------------------

The [Swarming bot
metrics](https://skia.googlesource.com/buildbot/+/master/datahopper/go/swarming_metrics/bots.go)
goroutine has not successfully queried for Swarming bots for some time. See the
alert for which pool and server is failing. You should check the
[logs](https://console.cloud.google.com/logs/viewer?project=google.com:skia-buildbots&minLogLevel=500&expandAll=false&resource=logging_log%2Fname%2Fskia-datahopper2&logName=projects%2Fgoogle.com:skia-buildbots%2Flogs%2Fdatahopper)
to try to diagnose what's failing.
