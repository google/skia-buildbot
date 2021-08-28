# Role Name

`copy_adbkey`

Copy the adbkey files to the chrome-bot home directory.

Does this safely by extracting the files from berglas to temp files, copying
them over, and then deleting the temp files.

## Role Name

`copy_adbkey`

## Description

Loads the common `adbkey` and `adbkey.pub` files from
[berglas](https://github.com/GoogleCloudPlatform/berglas) and copies them over
to the target machine. See
[//kube/secrets](https://skia.googlesource.com/buildbot/+/refs/heads/main/kube/secrets/)
for more details on berglas and Skia secrets.

The secrets are stored as kubernetes secrest in berglas secrets for the cluster
`etc` and the secret name `skia-rpi-adb-key`.

You can see this secret in the list of all secrets for the `etc` cluster:

        $ ../../kube/secrets/list-secrets-by-cluster.sh etc
        skolo-service-accounts
        skolo-bot-service-account
        skia-rpi-adb-key
        k3s-node-token
        authorized_keys
        ansible-secret-vars

## Variables

This role uses the `skolo_account` variable defined in `hosts.yml`.

## Security

The `secrets.yml` is only put in a temp file long enough to be copied to the
target machine, then the temp file is removed by the `clean_up_tempfile`
handler.

## Example Playbook

    - hosts: rpis

      roles:
        - copy_adbkey
