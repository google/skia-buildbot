# Role Name

`install_router_backup`

## Description

Builds and deploys the router_backup_ansible executable.

## Requirements

The default service account key installed for 'chrome-bot' must have the ability
to write logs to `skia-public` and to write files to the `gs://skia-backups`
bucket.

The jumphost must also have passwordless SSH access into the router, which is
always presumed to be at 192.168.1.1 on every rack.

## Example Playbook

    - hosts: jumphosts

      roles:
        - install_router_backup
