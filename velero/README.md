Velero Backup
=============

Velero is used to create and restore backups of secrets and configmaps in all
k8s clusters.

N.B.
----

We use the default 'velero install', but we store the key for the service
account in k8s secrets just like all our other service accounts.
The install-velero.sh script will copy the secret back out skia-public
secrets and pass it to the 'velero install' command line.

   velero install \
    --provider gcp \
    --bucket $BUCKET \
    --secret-file ./credentials-velero \
    --dry-run -o yaml
