# New Instance Checklist

When launching a new Perf instance:

## 1. Create a GCS Bucket

This is the bucket where Skia formatted JSONs are to be uploaded to trigger their ingestion.
These buckets live in the skia-public project, so you first need to have Storage Admin role in
this project in order to create and write to buckets.

Create bucket command example:

```
$ gcloud storage buckets create gs://flutter-skia-perf-prod --location=us
--uniform-bucket-level-access --project=skia-public
```

Once created, it's recommended to create a folder specifically for ingestion e.g.
`gs://flutter-skia-perf-prod/ingest`.

## 2. Create new database in CockroachDB.

This needs to be done from a machine on corp and also requires
[breakglass](https://grants.corp.google.com/#/grants) to the `skia-infra-breakglass-policy` group.
Note that there is a different connect script for each cluster.

Note: if you're creating a Googler-only instance, use `skia-infra-corp` instead
of `skia-infra-public`.

Port-forward the database:

```
$ ./kube/attach.sh skia-infra-public
kubectl port-forward --namespace=perf perf-cockroachdb-0 25000:26257
```

From another shell on the same computer connect to the database:

```
cockroach sql --insecure --host=127.0.0.1:25000
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

## 5. Create the PubSub topic and subscription for ingestion.

This creates the topic and subscription.

```
perf-tool config create-pubsub-topics-and-subscriptions --config_filename=./configs/angle.json
```

## 6. Configure GCS to emit PubSub Events:

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

gsutil notification create -f json -e OBJECT_FINALIZE -t projects/${PROJECT_ID}/topics/${TOPIC}
-p ${DIRECTORY} ${BUCKET}
```

Note that for buckets not owned by the Skia Infra team this command needs to be
run by someone with admin rights on the bucket and also the ability to create
the link to the pubsub receiver in the `skia-public` project. For non-Skia Infra
buckets I've found the easiest thing to do is give the requester privileges to
the `skia-public` project (for an hour) and have them run the above command.

## 7. Create a Service account for your instance

Create a CL like [this one](https://critique.corp.google.com/cl/568682178) to create a service
account. Make sure it's in the correct project.

Give this service account read access to the bucket created in step 1 and Pub/Sub Editor role to
both the topic and subscription created in step 5.

## 8. Start new "perfserver ingest" instances for the given data with new service account.

In `k8s-config` repo, create a CL like
[this one](https://skia-review.googlesource.com/c/k8s-config/+/759064), where you create a
`*-sa.yaml` file which points to the service account created in step 7, and a `*-ingest-*.yaml`
file where you define the ingestor specs for your perf instance. In the ingest file, make sure
to add the appropriate values for `app`, `serviceAccountName` and `--config_filename`.

## 9. [Optional] Use perf-tool to forcibly trigger re-ingestion of existing files.

```
perf-tool ingest force-reingest --config_filename=./configs/flutter-flutter2.json
```

## 10. Once data has been ingested stand up the "perfserver frontend" instance.

In `k8s-config` repo, create a CL like
[this one](https://skia-review.googlesource.com/c/k8s-config/+/761974), where you create a
`*-fe-*.yaml` file. Ensure to have appropriate values for `app`, `serviceAccountName`, `name`
and `--config_filename` flags.

Then run `skfe/generate.sh`. This will create the envoy config to route traffic to the instance.

## 11. Update the Skia zone file.

Add the sub-domain of the new Perf instance to the zone file and run:

./update-zone-records.sh

## 12. Add probers for the frontend.

In `k8s-config` repo, create a CL like
[this one](https://skia-review.googlesource.com/c/k8s-config/+/762921). Modify `perf.json` to
include your instance's URL. Then run `prober/generate.sh` to update `allprobersk.json` file.