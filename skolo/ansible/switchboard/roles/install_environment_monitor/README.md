# Role Name

`install_environment_monitor`

## Description

Builds and deploys the environment_monitor_ansible executable.

## Arguments

environment_monitor_ansible_version - Optional argument to select the
version of environment_monitor_ansible to install. If not set then the
version recorded in the k8s-config repo will be used.

## Example Playbook

```yaml
- hosts: environment_monitor_linux
  roles:
    - install_environment_monitor
```

## Pushing a test/debug binary:

To deploy a test/debug binary to a machine first upload the CIPD package via the
//skolo Makefile:

```console
$ cd skolo
$ make build_and_upload_environment_monitor_ansible
```

The logs from the build_and_upload command will contain the CIPD version for
that build. Pass that version to the ansible-playbook via --extra-vars. You
probably also want to only push your new configuration to a single jumphost at
first, using the limit.

You will run a command like this from //skolo/ansible, as per usual with ansible
playbooks:

```console
$ ansible-playbook ./switchboard/jumphosts.yml --limit rack5 \
  --extra-vars environment_monitor_ansible_version_override=2021-09-19T15:36:31Z-cmumford-ba7510fdcda7d3979cc2c0df21fee100e3ba4075-dirty
```

You can view the logs as they are streamed to [Cloud Logging](https://console.cloud.google.com/logs/viewer?project=skia-public&advancedFilter=logName%3D%22projects%2Fskia-public%2Flogs%2Fenvironment_monitor_ansible%22)

## Creating and deploying a new release.

Start with a clean and up-to-date workspace.

### Create a new release:

```console
# within the //skolo directory
$ make release_environment_monitor_ansible
```

Verify in the output that a "clean" image was generated. There should see something similar to:

```
[update-version 940ebd7e5c] Update Ansible version of environment_monitor_ansible to 2023-05-22T20_19_29Z-cmumford-3dd1051-clean.
```

### Deploy the new release:

To deploy the new release to all jumphosts:

```console
$ cd ansible
$ ansible-playbook switchboard/jumphosts.yml
```
