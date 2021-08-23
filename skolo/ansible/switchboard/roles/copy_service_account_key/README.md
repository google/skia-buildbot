# Role Name

`copy_service_account_key`

## Description

Copy the service account key to the chrome-bot home directory.

Does this safely by extracting the files from berglas to temp files, copying
them over, and then deleting the temp files.

Loads the common `key.json` file from
[berglas](https://github.com/GoogleCloudPlatform/berglas) and copies them over
to the target machine. See
[//kube/secrets](https://skia.googlesource.com/buildbot/+/refs/heads/main/kube/secrets/)
for more details on berglas and Skia secrets.

The key is stored as a kubernetes secret in berglas secrets for the cluster
`etc` and the secret name is passed in via the `copy_service_account_key_name`
variable.

You can see all secrets for the `etc` cluster by running:

        $ ../../kube/secrets/list-secrets-by-cluster.sh etc
        skolo-service-accounts
        skolo-bot-service-account
        skia-rpi-adb-key
        k3s-node-token
        authorized_keys
        ansible-secret-vars

The file is copied into
`$HOME/.config/gcloud/application_default_credentials.json` so that client
libraries can find and use this by default.

## Variables

This role uses the `skolo_account` variable defined in `hosts.ini`.

The `copy_service_account_key__name` is the name of the berglas secret that
contains the service account key to use.

The `copy_service_account_key.name` is the name of the berglas secret that
contains the service account key to use.

## Security

The `secrets.yml` is only put in a temp file long enough to be copied to the
target machine, then the temp file is removed by the `clean_up_tempfile`
handler.

## Example Playbook

    - hosts: rpis

      roles:
        - copy_service_account_key
