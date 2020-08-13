# New Instance Checklist

When launching a new Perf instance:

1. Create new database in CockroachDB.
1. Perform the migrations on the new database to create the tables. See COCKROACH.md.
1. Add the database to be backed up to `./images/backup/backup.sh`.
1. Push a new version of `perf-cockroachdb-backup`.
   - `make push_backup`
1. Add a script to create a new service account in secrets with access to the
   Google Cloud Storge location containing the files to ingest.
1. Add the secrets to the cluster.
1. Start new "perfserver ingest" instances for the given data with new service account.
1. Once data has been ingested stand up the "perfserver frontend" instance.
1. Add probers for the frontend.
