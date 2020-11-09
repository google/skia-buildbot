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
This alert indicates there are many tasks in the
[queue](https://ct.skia.org/queue/). There are several possibilities:

- CT may not have enough capacity to handle the current task requests. If there
  are many bare-metal tasks (these currently include ChromiumPerf tasks as well
  as ChromiumAnalysis tasks where `RunOnGCE` is
  `false`) requested in a short period of time, it may take a while to complete
  all tasks.
- Check the "Task Details" of each task in the
  [queue](https://ct.skia.org/queue/) for `"TsStarted": 0` (ignoring "scheduled
  in the future" tasks). CT normally picks up tasks in < 1m, so if a task is not
  started, that could mean that the CT poller is down (see below) or that
  something is wrong with the CT framework possibly related to a recent push.
- Check the status of the bots in the [CT SwarmingPool](
  https://chrome-swarming.appspot.com/botlist?c=id&c=os&c=task&c=status&f=pool%3ACT&l=100&s=id%3Aasc).
  * Note that the GCE bots will be dead if all pending tasks are bare-metal (see
    above for which tasks are bare-metal).
  * If many build*-m5 bots are dead, investigate why the bots are dead and/or
    [file a bug](
    https://code.google.com/p/chromium/issues/entry?template=Build%20Infrastructure)
    with ChOps (Chrome infra team).
  * If many bots are idle, check the Swarming task logs (see next bullet) to see
    if the dimensions of the pending tasks match the bot dimensions.
- Open the `SwarmingLogs` link shown in the "Task Details." If `build_chromium`
  has been running for > 1h, something is probably wrong.

