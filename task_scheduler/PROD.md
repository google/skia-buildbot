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


old_db_backup
-------------

The most recent backup of the local BoltDB database on Google Storage is more
than 25 hours old.

 - If db_backup_trigger_liveness is firing, resolve that first.

 - Look for backup files in the
   [skia-task-scheduler bucket](https://pantheon.corp.google.com/storage/browser/skia-task-scheduler/db-backup/)
   that are more recent, in case the alert is incorrect.

 - Check that task-scheduler-db-backup is deployed to the server and the systemd
   service is enabled.

 - Check if there are any files in the directory
   /mnt/pd0/task_scheduler_workdir/trigger-backup. If not, check the systemd
   logs for task-scheduler-db-backup for errors.

 - Check logs for "Automatic DB backup failed" or other errors.


too_many_recent_db_backups
--------------------------

There are too many recent backups in the
[skia-task-scheduler bucket](https://pantheon.corp.google.com/storage/browser/skia-task-scheduler/db-backup/).
This indicates a runaway process is creating unnecessary backups. Review the
task scheduler logs for "Beginning manual DB backup" to determine what is
triggering the excessive backups.


db_backup_trigger_liveness
--------------------------

The function DBBackup.Tick is not being called periodically. If
scheduling_failed alert is firing, resolve that first. Otherwise, check for
recent code changes that may have unintentionally removed the callback to
trigger a DB backup from the task scheduler loop.


db_too_many_free_pages
----------------------

The number of cached free pages in the Task Scheduler BoltDB has grown
large. As this number grows, DB performance suffers. Please file a bug and
increase the threshhold in alerts.cfg. It's unclear what causes this issue, but
it might be due to killing the process without gracefully closing the DB or due
to large read transactions concurrent with write transactions.
