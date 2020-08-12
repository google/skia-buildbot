# New Instance Checklist

When launching a new Perf instance:

1. Create new database in CockroachDB.
1. Add the database and tables to be backed up to `./images/backup/backup.sh`.
1. Push a new version of `perf-cockroachdb-backup`.
1. Add a script to create a new service account in secrets with access to the
   Google Cloud Storge location containing the files to ingest.
1. Add the secrets to the cluster.
1. Start new "perfserver ingest" instances for the given data.
1. Once data has been ingested stand up the "perfserver frontend" instance.
