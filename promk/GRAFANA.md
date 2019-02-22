Grafana
=======

Backups of the sqlite database for Grafana are handled by
the backup-to-gcs container that runs in the grafana-0 pod.
Once a day the database is copied into:

    gs://skia-public-backup/YYYY/MM/DD/grafana.db


