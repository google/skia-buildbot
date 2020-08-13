Task Scheduler Production Manual
================================

General information about the Task Scheduler is available in the
[README](./README.md).

GS bucket lifecycle config
--------------------------

The file bucket-lifecycle-config.json configures a Google Storage bucket to move
files in the skia-task-scheduler bucket to nearline or coldline storage after a
period of time. The configuration can be set by running `gsutil lifecycle set
bucket-lifecycle-config.json gs://skia-task-scheduler`.

[More documentation of object lifecycle](https://cloud.google.com/storage/docs/lifecycle).


Troubleshooting
===============

git-related errors in the log
-----------------------------

Eg. fatal: unable to access '$REPO': The requested URL returned error: 502

Unfortunately, these are pretty common, especially in the early afternoon when
googlesource is under load. Usually they manifest as a 502, or "repository not
found". If these are occurring at an unusually high rate (more than one or two
per hour) or the errors look different, contact an admin and ask if there are
any known issues: http://go/gob-oncall


Extremely slow startup
----------------------

Ie. more than a few minutes before the server starts responding to requests, or
liveness_last_successful_task_scheduling_s greater than a few minutes
immediately after server startup.

The task scheduler has to load a lot of data from the DB on startup.
Additionally, each of its clients reloads all of its data when the scheduler
goes offline and comes back. These long-running reads can interact with writes
such that operations get blocked and continue piling up. Requests time out and
are retried, compounding the problem. If you notice this happening (an extremely
long list of "task_scheduler_db Active Transactions" in the log is a clue), you
can ease the load on the scheduler by shutting down both Status and Datahopper,
restart the scheduler and wait until it is up and running, then restart Status,
wait until it is up and running, and finally restart Datahopper. In each case,
watch the logs and ensure that all "Reading Tasks from $start_ts to $end_ts"
have completed successfully.
TODO(borenet): This should not be necessary with the new DB implementation.


DB is very slow
---------------

Eg. liveness_last_successful_task_scheduling_s is consistently greater than a
few minutes, and "task_scheduler_db Active Transactions" in the log are piling
up.

Similar to the above, but not caused by long-running reads. BoltDB performance
can degrade for a number of reasons, including excessive free pages. A potential
fix is to stop the scheduler and run "bolt compact" over the database file.
TODO(borenet): This should not be necessary with the new DB implementation.


Alerts
======

scheduling_failed
-----------------

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


old_db_backup
-------------

The most recent backup of the local BoltDB database on Google Storage is more
than 25 hours old.

 - If db_backup_trigger_liveness is firing, resolve that first.

 - Look for backup files in the
   [skia-task-scheduler bucket](https://console.cloud.google.com/storage/browser/skia-task-scheduler/db-backup/)
   that are more recent, in case the alert is incorrect.

 - Check that task-scheduler-db-backup is deployed to the server and the systemd
   service is enabled.

 - Check if there are any files in the directory
   `/mnt/pd0/task_scheduler_workdir/trigger-backup`. If not, check the systemd
   logs for task-scheduler-db-backup for errors.

 - If the systemd timer failed to execute, you can trigger a manual
   backup by running `touch
   /mnt/pd0/task_scheduler_workdir/trigger-backup/task-scheduler-manual`.

 - Otherwise, check logs for "Automatic DB backup failed" or other errors.


too_many_recent_db_backups
--------------------------

There are too many recent backups in the
[skia-task-scheduler bucket](https://console.cloud.google.com/storage/browser/skia-task-scheduler/db-backup/).
This indicates a runaway process is creating unnecessary backups. Review the
task scheduler logs for "Beginning manual DB backup" to determine what is
triggering the excessive backups.


db_backup_trigger_liveness
--------------------------

The function DBBackup.Tick is not being called periodically. If
scheduling_failed alert is firing, resolve that first. Otherwise, check for
recent code changes that may have unintentionally removed the callback to
trigger a DB backup from the task scheduler loop.


incremental_backup_liveness
---------------------------

The function gsDBBackup.incrementalBackupStep has not succeeded recently. Check
logs for "Incremental Job backup failed". If Task Scheduler is otherwise
operating normally, this is not a critical alert, since we also perform a full
nightly backup.


incremental_backup_reset
------------------------

The function gsDBBackup.incrementalBackupStep is not able to keep up with the
rate of new and modified Jobs. This likely indicates a problem with the
connection to Google Storage or the need for additional concurrency. Check logs
for "Incremental Job backup failed" or "incrementalBackupStep too slow". This
alert will also resolve itself after the next full backup, which can be manually
triggered by running `touch
/mnt/pd0/task_scheduler_workdir/trigger-backup/task-scheduler-manual`.


db_too_many_free_pages
----------------------

The number of cached free pages in the Task Scheduler BoltDB has grown
large. As this number grows, DB performance suffers. Please file a bug and
increase the threshold in alerts.cfg. It's unclear what causes this issue, but
it might be due to killing the process without gracefully closing the DB or due
to large read transactions concurrent with write transactions.


too_many_candidates
-------------------

The number of task candidates for a given dimension set is very high. This may
not actually indicate that anything is wrong with the Task Scheduler. Instead,
it may just mean that demand has exceeded bot capacity for one or more types of
bots for an extended period. If possible, increase the bot capacity by adding
more bots or by fixing offline or quarantined bots. Consider temporarily
skipping backfill tasks for these bots to reduce load on the scheduler. An
alternative long-term fix is to remove tasks for overloaded bots.


trigger_nightly
---------------

The nightly trigger has not run in over 25 hours. Check that the
task-scheduler-trigger-nightly.service has run. If not, check the systemctl
settings on the server. If so, check the Task Scheduler logs.


trigger_weekly
--------------

The weekly trigger has not run in over 8 days. Check that the
task-scheduler-trigger-weekly.service has run. If not, check the systemctl
settings on the server. If so, check the Task Scheduler logs.


overdue_metrics_liveness
------------------------

The function TaskScheduler.updateOverdueJobSpecMetrics is not being called
periodically. If scheduling_failed alert is firing, resolve that first.
Otherwise, check the logs for error messages, check the
`timer_func_timer_ns{func="updateOverdueJobSpecMetrics"}` metric, or look
for recent changes that may have affected this function.


overdue_job_spec
----------------

Tasks have not completed recently for the indicated job, even though a
reasonable amount of time has elapsed since an eligible commit. If any other
task scheduler alerts are firing, resolve those first. Otherwise:

 - Check Status for pending or running tasks for this job. The Swarming UI
   provides the best information on why the task has not completed.

 - Check that the dimensions specified for the job's tasks match the bot that
   should run those tasks.

 - Check that the bots are available to run the tasks. Remember that forced jobs
   will always be completed before other jobs, and tryjobs get a higher score
   than regular jobs.

    - If there are many forced jobs that were triggered accidentally, the [Job
      search UI](https://task-scheduler.skia.org/jobs/search) can be used to
      bulk-cancel jobs.


latest_job_age
--------------

Jobs have not been triggered recently enough for the indicated job spec. This
normally indicates that the periodic triggers have stopped working for some
reason. Double check that the "periodic-trigger" cron jobs have run at the
expected time in Kubernetes. If they have not, look into why. If they have,
check the Task Scheduler logs to verify that the scheduler received the pubsub
message and if so determine why it did not create the job.


update_repos_failed
-------------------

The scheduler (job creator) has failed to update its git repos for too long.
Check the logs and determine what is going on. If the git servers are down or
having problems, make sure that the team is aware by filing a bug or pinging
IRC:
http://go/gob-oncall


poll_buildbucket_failed
-----------------------

The scheduler (job creator) has not successfully polled Buildbucket for new
tryjobs in a while. Any tryjobs started by the CQ or manually during this period
have not been picked up yet. Check the logs and determine what is going on.

If the git servers are down or having problems, make sure that the team is aware
by filing a bug or pinging IRC:
http://go/gob-oncall

If Buildbucket is down or having problems, see https://g.co/bugatrooper

You may want to notify skia-team about the disruption.


update_buildbucket_failed
-------------------------

The scheduler (job creator) has not successfully sent heartbeats to Buildbucket
for in-progress tryjobs or sent status updates to Buildbucket for completed
tryjobs in a while. Any tryjobs that have completed during this time will not be
reflected on Gerrit or the CQ. Check the logs and determine what is going on.

If Buildbucket is down or having problems, see https://g.co/bugatrooper

You may want to notify skia-team about the disruption.
