# New Instance Checklist

When launching a new Perf instance:

## 1. Create new database in CockroachDB.

This needs to be done from a machine on corp and also requires breakglass. Note
that there is a different connect script for each cluster.

Port-forward the database:

```
$ ./kube/attach.sh skia-infra-public
kubectl port-forward --namespace=perf  perf-cockroachdb-0 26257
```

From another shell on the same computer connect to the database:

```
cockroach sql --insecure --host=127.0.0.1:26257
```

Create the database:

```
root@perf-cockroachdb-public:26257/defaultdb> CREATE DATABASE flutter_flutter2;
CREATE DATABASE

Time: 24.075052ms
```

Use the newly created database:

```
root@perf-cockroachdb-public:26257/defaultdb> use flutter_flutter2;
```

Apply the schema to the database. The schema is found
in `//perf/go/sql/schema.go.`

```
root@perf-cockroachdb-public:26257/defaultdb> CREATE TABLE ...
```

## 3. Add the database to be backed up to `./backup/backup.sh`.

## 4. Push a new version of `perf-cockroachdb-backup`.

    make push_backup

## 5. Service account

Make sure the workload identity service account used for the running
Perf instance has read access to the bucket the ingesters are reading from.

## 6. Create the PubSub topic for ingestion.

This creates the topic.

```
perf-tool config create-pubsub-topics --config_filename=./configs/angle.json
```

## 7. Configure GCS to emit PubSub Events:

This configures the GCS bucket/directory to send PubSub events to that topic
when new files arrive:

```
#/bin/bash
# Creates the PubSub topic for Android Perf files and then ties it to GCS
# notifications.

set -e -x

PROJECT_ID=skia-public
TOPIC=perf-ingestion-flutter-flutter2
DIRECTORY=flutter-flutter
BUCKET=gs://flutter-skia-perf-prod

gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC} -p ${DIRECTORY} ${BUCKET}
```

Note that for buckets not owned by the Skia Infra team this command needs to be
run by someone with admin rights on the bucket and also the ability to create
the link to the pubsub receiver in the `skia-public` project. For non-Skia Infra
buckets I've found the easiest thing to do is give the requester privileges to
the `skia-public` project (for an hour) and have them run the above command.

## 8. Start new "perfserver ingest" instances for the given data with new service account.

## 9. [Optional] Use perf-tool to forcibly trigger re-ingestion of existing files.

```
perf-tool ingest force-reingest --config_filename=./configs/flutter-flutter2.json
```

## 10. Once data has been ingested stand up the "perfserver frontend" instance.

This will also create the envoy config to route traffic to the instance.

## 11. Update the Skia zone file.

Add the sub-domain of the new Perf instance to the zone file and run:

./update-zone-records.sh

## 12. Add probers for the frontend.
