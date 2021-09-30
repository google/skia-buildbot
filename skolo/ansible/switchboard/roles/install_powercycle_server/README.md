# Role Name

`install_powercycle_server`

## Description

Builds and deploys the powercycle_server_ansible executable.

The server uses the Google Default Application Credentials for the `chrome-bot`
user.

## Requirements

The default service account key installed for 'chrome-bot' must have the ability
to write logs to `skia-public` and access the machine server Firebase database.

## Arguments

powercycle_server_ansible_version - Optional argument to select the version of
powercycle_server_ansible to install. If not set then the version recorded in
the k8s-config repo will be used.

## Variables Required

This role requires the `secrets.skolo_password`, which is loaded via the
required role `load_secret_vars`.

## Example Playbook

    - hosts: jumphosts

      roles:
        - install_powercycle_server

## Pushing a test/debug binary:

To deploy a test/debug binary to a machine first upload the cipd package via the
//skolo Makefile:

```
$ cd skolo
$ make build_powercycle_server_ansible
```

Then visit http://go/cipd/p/skia/internal/powercycle_server_ansible/+/ to find
the version for that build and pass it to a playbook via --extra-vars.

For example:

```
$ ansible-playbook ./switchboard/jumphosts.yml \
  --extra-vars powercycle_server_ansible_version_override=2021-09-19T15:36:31Z-jcgregorio-ba7510fdcda7d3979cc2c0df21fee100e3ba4075-dirty
```
