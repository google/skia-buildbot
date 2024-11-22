# Role Name

`copy_service_account_key`

## Description

Copy the service account key to the chrome-bot home directory.

Does this safely by downloading the key from GCP Secrets to temp files, copying
them over, and then deleting the temp files.

The file is copied into
`$HOME/.config/gcloud/application_default_credentials.json` so that client
libraries can find and use this by default.

## Variables

This role uses the `skolo_account` variable defined in `hosts.yml`.

The `copy_service_account_key__name` is the name of the service account, and
`copy_service_account_key__project` is the project in which the service account
is defined.

## Security

The `secrets.yml` is only put in a temp file long enough to be copied to the
target machine, then the temp file is removed by the `clean_up_tempfile`
handler.

## Example Playbook

    - hosts: rpis

      roles:
        - copy_service_account_key
