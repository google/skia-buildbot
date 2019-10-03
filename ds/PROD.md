Datastore Backup Production Manual
==================================

General info is available in the [README](./README.md)


Alerts
======

backup_not_done
---------------

Backups should run once a week. Restarting datastore_backup should immediately
trigger a backup, confirm this by looking in the logs and also by inspecting
gs://skia-backups/ds/ for a new set of backups.

boot_loop
=========

Check the logs for why datastore_backup is restarting, or the liveness metric
isn't updating.

