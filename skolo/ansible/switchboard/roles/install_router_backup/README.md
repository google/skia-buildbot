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

## Arguments

install_router_backup_version - Optional argument to select the version of
router_backup_ansible to install. If not set then the version recorded in the
k8s-config repo will be used.

## Example Playbook

    - hosts: jumphosts

      roles:
        - install_router_backup

## Pushing a test/debug binary:

To deploy a test/debug binary to a machine first upload the cipd package via the
//skolo Makefile:

```
$ cd skolo
$ make build_router_backup_ansible
```

Then visit http://go/cipd/p/skia/internal/router_backup_ansible/+/ to find the
version for that build and pass it to a playbook via --extra-vars.

For example:

```
$ ansible-playbook ./switchboard/jumphosts.yml \
  --extra-vars router_backup_ansible_version_override=2021-09-19T15:36:31Z-jcgregorio-ba7510fdcda7d3979cc2c0df21fee100e3ba4075-dirty
```
