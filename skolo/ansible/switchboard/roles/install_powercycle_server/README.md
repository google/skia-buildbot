# Role Name

`install_powercycle_server`

## Description

Builds and deploys the powercycle_server_ansible executable.

The server uses the Google Default Application Credentials for the `chrome-bot`
user.

## Requirements

The default service account key installed for 'chrome-bot' must have the ability
to write logs to `skia-public` and access the machine server Firebase database.

## Variables Required

This role requires the `secrets.skolo_password`, which is loaded via the
required role `load_secret_vars`.

## Example Playbook

    - hosts: jumphosts

      roles:
        - install_powercycle_server
