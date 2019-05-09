Velero Backup
=============

Velero is used to create and restore backups of secrets and configmaps in all
k8s clusters.

N.B.
----

We do not use the default 'velero install' because we want to check in the
`velero.yaml` file, and Velero just puts the service account secret as base64
encoded text in the YAML file, which is a really bad idea. So we run install
with the --dry-run flag, hand modify the yaml file to remove the inline secret
and manually add in the backup schedule and backup options, then copy that
`velero.yaml` file into the correct git repo:

   velero install \
    --provider gcp \
    --bucket $BUCKET \
    --secret-file ./credentials-velero \
    --dry-run -o yaml
