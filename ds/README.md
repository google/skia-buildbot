# Datastore backups

This is the application used to back up Cloud Datastore entities to Cloud Storage.

See: https://cloud.google.com/datastore/docs/schedule-export

Backups are made to gs://skia-backups/ds/YYYY/MM/DD/HH

Restoring from a backup can be done via the gcloud command line tool.
See https://cloud.google.com/datastore/docs/export-import-entities

## Locations

The bucket has to live in the same project as the datastore data.

The buckets are:

  * `gs://skia-datastore-backups` for `skia-public`
  * `gs://skia-datastore-backups-skia-buildbots` for `google.com:skia-buildbots`
  * `gs://skia-datastore-backups-skia-corp` for `skia-corp`

To see all the running backups run the following on the `skia-public` cluster:

    kubectl get pods -lappgroup=datastore-backup
