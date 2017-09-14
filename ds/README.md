This is the application used to back up Cloud Datastore entities to Cloud Storage.

See: https://cloud.google.com/datastore/docs/schedule-export

Backups are made to gs://skia-backups/ds/YYYY/MM/DD/HH

Restoring from a backup can be done via the gcloud command line tool.
See https://cloud.google.com/datastore/docs/export-import-entities

