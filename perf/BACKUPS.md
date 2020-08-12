# Backups

Backsups are done via `cockrachdb dump` for the tables that need to be
backed up.

In this case only the Alerts table is backed up.

The remaining tables are not backed up:

- Commits - Is recreated from git as the source of truth on startup.
- Regressions - Regressions for the last 200 commits are automatically built
  by the clustering process.
- All trace data tables can be rebuilt by re-ingesting data from Google Cloud Storage.
- Shortcuts are too numerous to backup and they are only really used
  in Regressions which get rebuilt anyway.

Maybe in the future shourtcuts could have a created/last used time and a subset
of them could be backed up.

To make the system as simple as possible a single script runs once a day, dumps
the Alerts tables from all the instances, and then copies those backups to GCS.

We need a docker image that contains `cockroachdb`, `gsutil`, and `gzip`.
