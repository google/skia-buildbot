# Backups

Backups are done via `perf-tool database backup` for the regressions and alerts.

There are things that are not backed up:

- Commits - Is recreated from git as the source of truth on startup if the table is empty.
- All trace data tables can be rebuilt by re-ingesting data from Google Cloud Storage.
- Shortcuts are too numerous to backup and they are only really used in
  Regressions which get rebuilt anyway. The shortcuts used by Regressions are
  backed up with Regressions.

To make the system as simple as possible a single script runs once a day that
uses `perf-tool database backup` and then copies those gzipped backups to GCS.

We use a docker image that contains `perf-tool` and `gsutil`.

## Adding a new database to backup.

Edit `./images/backup/backup.sh` to add the new config name and then:

    $ make push_backup

## Testing backups

To test backups you can manually create a job from the cronjob:

    $ kubectl create job --from=cronjob/perf-cockroachdb-backup-daily cdb-backup-manual-005

And then watch the logs for that job as it runs:

    $ kubectl logs -f job/cdb-backup-manual-005

## Restoring

- Download the backup files.
- Port forward the CockroachDB instance to localhost:

```
kubectl port-forward perf-cockroachdb-0 26257`
```

- Restore the backups:

```
perf-tool database restore alerts      --config=$config --in=alerts.dat      --connection_string=$connection
perf-tool database restore regressions --config=$config --in=regressions.dat --connection_string=$connection
perf-tool database restore shortcuts   --config=$config --in=regressions.dat --connection_string=$connection
```
