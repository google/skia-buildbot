# New Instance Checklist

When launching a new Perf instance, first determine whether you are creating a Googler-only
or a publicly available instance. Based on that, all resources except the GCS bucket will
be created in the respective projects mentioned below.

| Instance Type | GCP Project       |
| ------------- | ----------------- |
| Googlers-only | skia-infra-corp   |
| Public        | skia-infra-public |

## 1. Create a GCS Bucket

This is the bucket where Skia formatted JSONs are to be uploaded to trigger their ingestion.
These buckets live in the skia-public project. To create a new bucket, update [this
terraform file](go/skia-perf-buckets)

Determine which service account will be writing the input files into this bucket and provide
that account with `objectAdmins` permission.

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

Run the below cmd from the **perf** directory.

    make push_backup

## 5. Create the PubSub topic and subscription for ingestion.

This creates the topic and subscription.

```
perf-tool config create-pubsub-topics-and-subscriptions --config_filename=./configs/angle.json
```

## 6. Configure GCS to emit PubSub Events:

This configures the GCS bucket/directory to send PubSub events to that topic
when new files arrive. Update the variable values based on the output of the above step:

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

## 7. Create a GCP Service account for your instance

Create a CL like [this one](http://go/sample-sa-cl) to create a service
account. Make sure it's in the correct project.

Give this service account read access to the bucket created in step 1 and Pub/Sub Editor role to
both the topic and subscription created in step 5.

If you are creating a Googlers-only instance, the service accounts needs to be added to the
auth-proxy roster so that it can access secrets in the GCP project. Create a CL like
[this one](http://go/sample-auth-proxy-roster-cl) to do the same.

## 8. Bind the GCP service account to a Kubernetes service account in the cluster

In `k8s-config` repo, create a CL with a `*-sa.yaml` file which points to the service account created in step 7:

```
apiVersion: v1
kind: ServiceAccount
metadata:
  annotations:
    iam.gke.io/gcp-service-account: perf-<XXXX>@<CLUSTER>.iam.gserviceaccount.com
  name: perf-<XXXX>-internal
  namespace: perf
```

See [this file](https://skia-review.googlesource.com/c/k8s-config/+/759064/4/skia-infra-corp/perf-webrtc-internal-sa.yaml)
as an example.

## 9. Start new "perfserver maintenance" instance for the given data with new service account.

In `k8s-config` repo, create a CL like [this
one](https://skia-review.googlesource.com/c/k8s-config/+/794378), where you
create a `*-maintenance-*.yaml` file where you define the maintenance task for
your perf instance.

## 10. Start new "perfserver ingest" instances for the given data with new service account.

In `k8s-config` repo, create a CL like
[this one](https://skia-review.googlesource.com/c/k8s-config/+/759064), where you create a
`*-sa.yaml` file which points to the service account created in step 7, and a `*-ingest-*.yaml`
file where you define the ingestor specs for your perf instance. In the ingest file, make sure
to add the appropriate values for `app`, `serviceAccountName` and `--config_filename`.

## 11. [Optional] Use perf-tool to forcibly trigger re-ingestion of existing files.

```
perf-tool ingest force-reingest --config_filename=./configs/flutter-flutter2.json
```

## 12. Once data has been ingested stand up the "perfserver frontend" instance.

In `k8s-config` repo, create a CL like
[this one](https://skia-review.googlesource.com/c/k8s-config/+/761974), where you create a
`*-fe-*.yaml` file. Ensure to have appropriate values for `app`, `serviceAccountName`, `name`
and `--config_filename` flags.

Then run `skfe/generate.sh`. This will create the envoy config to route traffic to the instance.

## 13. Update the DNS for the instance.

For skia-infra-public:

- No action needed.

For skia-infra-corp:

- Create a CL like [this one](http://go/sample-skiaperf-dns-cl) to update the DNS
  record for the new instance.
- Since skia-infra-corp is behind UberProxy, we need to add the new host name in the uberproxy ACL.
  Create an ACLAIM proposal like [this one](go/perf-uberproxy-aclaim). Once this is approved, it
  generally takes about 24 hours for it to propagate.

## 14. Add probers for the frontend.

In `k8s-config` repo, create a CL like
[this one](https://skia-review.googlesource.com/c/k8s-config/+/762921). Modify `perf.json` to
include your instance's URL. Then run `prober/generate.sh` to update `allprobersk.json` file.
