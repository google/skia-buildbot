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

The velero resources are all under the 'velero' namespace, so to see
them you need to speficy the namespace, or use --all-namespaces in
kubectl commands. For example:

    kubectl get pods --namespace=velero

Alerts
------

Prometheus alerts exist for each cluster to catch backups that fail.


Upgrading
---------

Running the install-\* scripts should install the correct version, but run:

    velero version

To make sure both the client and server agree. If not you might need to
manually set the server version, for example:

    kubectl -n velero set image deployment/velero velero=gcr.io/heptio-images/velero:v1.0.0-rc.1

