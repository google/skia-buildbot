# Role Name

`copy_adbkey`

Copy the adbkey files to the chrome-bot home directory.

Does this safely by downloading the keys from GCP Secrets to temp files, copying
them over, and then deleting the temp files.

## Role Name

`copy_adbkey`

## Description

Loads the common `adbkey` and `adbkey.pub` files from the GCP Secrets `skia-rpi-adb-key` and `skia-rpi-adb-key-pub`, respectively, and copies them
over to the target machine.

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
