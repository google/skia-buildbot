# New Instance Checklist

When launching a new Perf instance:

1. Create new database in CockroachDB.

```
$ ./cockroachdb/connect.sh
If you don't see a command prompt, try pressing enter.
root@perf-cockroachdb-public:26257/defaultdb> CREATE DATABASE flutter_flutter2;
CREATE DATABASE

Time: 24.075052ms
```

2. Perform the migrations on the new database to create the tables. See
   COCKROACH.md.

First port-forward in the production database:

```
kubectl port-forward perf-cockroachdb-0 26257:26257
```

Then apply the migrations:

```
$ perf-tool database migrate \
   --config_filename=./configs/flutter-flutter2.json  \
   --connection_string=postgresql://root@localhost:26257/flutter_flutter2?sslmode=disable
```

3. Add the database to be backed up to `./images/backup/backup.sh`.
4. Push a new version of `perf-cockroachdb-backup`.
   - `make push_backup`
5. **Optional**: Add a script to create a new service account in secrets with
   access to the Google Cloud Storage location containing the files to ingest.
   This step is optional if you are re-using an existing service account, such
   as `skia-perf-sa` for the new instance. Note that there may be different
   service accounts for ingestion vs front-end instances. For example
   perf-ingest@skia-public.iam.gserviceaccount.com is typically used for
   ingestion instances.

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

Create the secrets:

```
./secrets/create-flutter-perf-service-account.sh
```

Apply the secrets to the cluster.

```
../kube/secrets/apply-secret-to-cluster.sh skia-public flutter-perf-service-account
```

7. Create the PubSub topic for ingestion.

This creates the topic and also configures the GCS bucket/directory to send
PubSub events to that topic when new files arrive:

```
#/bin/bash
# Creates the PubSub topic for Android Perf files and then ties it to GCS
# notifications.

set -e -x

PROJECT_ID=skia-public
TOPIC=perf-ingestion-flutter-flutter2

perf-tool config create-pubsub-topics --config_filename=./configs/flutter.json
gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC} -p flutter-flutter gs://flutter-skia-perf-prod
```

8. Start new "perfserver ingest" instances for the given data with new service
   account.

Also make sure this has only 1 replica, so that two instances aren't both trying
to add the full repo history to the database at the same time.

9.  [Optional] Use perf-tool to forcibly trigger reingestion of existing files.

```
perf-tool ingest force-reingest --config_filename=./configs/flutter-flutter2.json
```

10. Once data has been ingested stand up the "perfserver frontend" instance.
11. Add probers for the frontend.
