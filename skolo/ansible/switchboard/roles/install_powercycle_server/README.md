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

To deploy a test/debug binary to a machine first upload the CIPD package via the
//skolo Makefile:

```
$ cd skolo
$ make build_and_upload_powercycle_server_ansible
```

The logs from the build_and_upload command will contain the CIPD version for that build.
Pass that version to the ansible-playbook via --extra-vars. You probably also want
to only push your new configuration to a single jumphost at first, using the limit.

You will run a command like this from //skolo/ansible, as per usual with ansible playbooks.:

```
$ ansible-playbook ./switchboard/jumphosts.yml --limit rack2 \
  --extra-vars powercycle_server_ansible_version_override=2021-09-19T15:36:31Z-jcgregorio-ba7510fdcda7d3979cc2c0df21fee100e3ba4075-dirty
```

You can view the logs as they are streamed to [Cloud Logging](https://console.cloud.google.com/logs/viewer?project=skia-public&advancedFilter=logName%3D%22projects%2Fskia-public%2Flogs%2Fpowercycle_server_ansible%22)
