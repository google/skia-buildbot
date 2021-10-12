# Role Name

`install_metadata_server`

## Description

Deploys the metadata_server_ansible executable.

The server is built using the `build_and_release_metadata_server_ansible.yml`
playbook, which builds and stores the binary in CIPD. This role pulls the latest
version from CIPD to deploy.

## Arguments

metadata_server_ansible_version - Optional argument to select the version of
metadata_server_ansible to install. If not set then the version recorded in the
k8s-config repo will be used.

## Requirements

The default service account key installed for 'chrome-bot' must have the ability
to write logs to `skia-public`.

## Security

The `key.json` is only put in a variable long enough to be embedded in the
executable.

## Example Playbook

    - hosts: jumphosts

    - role: install_metadata_server
      metadata_server_ansible_version:
        '{{ metadata_server_ansible_version_override }}'

## Pushing a test/debug binary:

To deploy a test/debug binary to a machine first upload the cipd package via the
`build_and_release_metadata_server_ansible.yml` playbook:

```
$ cd skolo/ansible
$ ansible-playbook ./switchboard/build_and_release_metadata_server_ansible.yml
```

Then visit http://go/cipd/p/skia/internal/metadata_server_ansible/+/ to find the
version for that build and pass it to a playbook via --extra-vars.

For example:

```
$ ansible-playbook ./switchboard/jumphosts.yml \
  --extra-vars metadata_server_ansible_version_override=2021-09-19T15:36:31Z-jcgregorio-ba7510fdcda7d3979cc2c0df21fee100e3ba4075-dirty
```
