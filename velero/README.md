Velero Backup
=============

Velero is used to create and restore backups of secrets and configmaps in all
k8s clusters.

We use the default 'velero install', but we store the key for the service
account in k8s secrets just like all our other service accounts. The
install-\*.sh script will copy the secret back out skia-public secrets and
pass it to the 'velero install' command line.

Usage
-----

Install the [velero command line application](https://github.com/heptio/velero/releases).

To check the status of a backup run:

    velero schedule describe

Each cluster should be scheduled to be backed up once a day. See
`create-schedule.sh` for those schedules.


Alerts
------

Prometheus alerts exist for each cluster to catch backups that fail.
