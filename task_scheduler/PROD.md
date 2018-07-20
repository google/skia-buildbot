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
blacklisting backfill tasks for these bots to reduce load on the scheduler. An
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
