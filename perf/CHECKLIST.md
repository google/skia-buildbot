# New Instance Checklist

When launching a new Perf instance:

1. Create new database in CockroachDB.
1. Perform the migrations on the new database to create the tables. See COCKROACH.md.
1. Add the database to be backed up to `./images/backup/backup.sh`.
1. Push a new version of `perf-cockroachdb-backup`.
   - `make push_backup`
1. Add a script to create a new service account in secrets with access to the
   Google Cloud Storge location containing the files to ingest.

```
#/bin/bash

# Creates the service account for flutter perf.
../../kube/secrets/add-service-account.sh \
  skia-public \
  skia-public \
  flutter-perf-service-account \
  "The flutter perf service account." \
  roles/pubsub.editor \
  roles/cloudtrace.agent
```

1. Add the secrets to the cluster.

```
./secrets/create-flutter-perf-service-account.sh
```

1. Create the PubSub topic for ingestion.

```
#/bin/bash
# Creates the PubSub topic for Android Perf files and then ties it to GCS
# notifications.

set -e -x

PROJECT_ID=skia-public
TOPIC=perf-ingestion-flutter-flutter2

perf-tool config create-pubsub-topics --config_filename=./configs/flutter.json
gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC} -p flutter-flutter gs://flutter-skia-perf
```

1. Have GCS create PubSub events that are sent to that topic when new files arrive in the bucket/directory.
1. Start new "perfserver ingest" instances for the given data with new service account.
1. [Optional] Use perf-tool to forcibly trigger reingestion of existing files.
1. Once data has been ingested stand up the "perfserver frontend" instance.
1. Add probers for the frontend.
