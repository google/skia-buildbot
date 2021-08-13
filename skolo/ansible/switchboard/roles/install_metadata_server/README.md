# Role Name

`install_metadata_server`

## Description

Builds and deploys the metadata_server_ansible executable.

The server is built with the service account key embedded into the executable.
It does this safely by extracting the key from berglas to a variable, and then
passing that variable to the build of the Go metadata_server_ansible executable,
which embeds the base64 encoded value in the executable via ldflags.

Loads the common `key.json` file from
[berglas](https://github.com/GoogleCloudPlatform/berglas) and loads that into an
Ansible variable. See
[//kube/secrets](https://skia.googlesource.com/buildbot/+/refs/heads/main/kube/secrets/)
for more details on berglas and Skia secrets.

The key is stored as a kubernetes secret in berglas secrets for the cluster
`etc` and the secret name `skolo-service-accounts`.

You can see this secret in the list of all secrets for the `etc` cluster:

        $ ../../kube/secrets/list-secrets-by-cluster.sh etc
        skolo-service-accounts
        skolo-bot-service-account
        skia-rpi-adb-key
        k3s-node-token
        authorized_keys
        ansible-secret-vars

## Requirements

The default service account key installed for 'chrome-bot' must have
the ability to write logs to `skia-public`.

## Security

The `key.json` is only put in a variable long enough to be embedded in the
executable.

## Example Playbook

    - hosts: jumphosts

      roles:
        - install_metadata_server
