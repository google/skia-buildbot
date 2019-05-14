Velero Production Manual
========================

Alerts
======

backup_failed
-------------

Confirm velero is running:

    kubectl get pods --namespace=velero

Check the status of the last backup run:

    velero schedule describe

Check the logs for backups:

    kubectl logs deployment/velero -n velero

