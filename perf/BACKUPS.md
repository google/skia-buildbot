# Backups

Backups are done via `cockroach dump` for the tables that need to be backed up.

In this case only the Alerts table is backed up.

The remaining tables are not backed up:

- Commits - Is recreated from git as the source of truth on startup if the table is empty.
- Regressions - Regressions for the last 200 commits are automatically built
  by the clustering process.
- All trace data tables can be rebuilt by re-ingesting data from Google Cloud Storage.
- Shortcuts are too numerous to backup and they are only really used
  in Regressions which get rebuilt anyway.

Maybe in the future shourtcuts could have a created/last used time and a subset
of them could be backed up.

To make the system as simple as possible a single script runs once a day, dumps
the Alerts tables from all the databases, and then copies those gzipped backups
to GCS.

We use a docker image that contains `cockroachdb`, `gsutil`, and `gzip`.

## Adding a new database to backup.

Edit `./images/backup/backup.sh` to add the new database name and then:

    $ make push_backup

## Testing backups

To test backups you can manually create a job from the cronjob:

    $ kubectl create job --from=cronjob/perf-cockroachdb-backup-daily cdb-backup-manual-005

And then watch the logs for that job as it runs:

    $ kubectl logs -f job/cdb-backup-manual-005

## Restoring

1. Download and unzip the backup.
1. Edit the file to remove the CREATE statement at the beginning.
1. Port forwared the CockroachDB instance to localhost:
  * `kubectl port-forward perf-cockroachdb-0 26257`
1. Execute the SQL statements inside the backup:
  * `cockroach sql --insecure --host=localhost < alerts-backup.sql `
