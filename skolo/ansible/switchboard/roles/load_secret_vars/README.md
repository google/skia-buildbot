# Role Name

Loads secrets from [berglas](https://github.com/GoogleCloudPlatform/berglas) and
makes them available as a variable. See
[//kube/secrets](https://skia.googlesource.com/buildbot/+/refs/heads/main/kube/secrets/)
for more details on berglas and Skia secrets.

The secrets are stored as a single file, `secrets.yml`, in berglas secrets for
the cluster `etc` and the secret name `ansible-secret-vars`.

You can see this secret in the list of all secrets for the `etc` cluster:

    $ ./kube/secrets/list-secrets-by-cluster.sh etc
    k3s-node-token
    authorized_keys
    ansible-secret-vars

## Editing

To edit the secrets run:

    kube/secrets/edit-secret.sh etc ansible-secret-vars

You can now edit the secrets stored in the file at `/tmp/ramdisk/secrets.yml`.

Add new secrets as `key: value` pairs of the top level `secrets` dictionary.

The only secret in the file today is `skolo_password`.

## Security

The `secrets.yml` is only put in a temp file long enough to be loaded into an
Ansible variable, then the temp file is removed by the `clean_up_tempfile`
handler.

## Example Playbook

    - hosts: '{{ variable_hosts }}'
      gather_facts: False

      roles:
        - load-secret-vars

      tasks:
        - name: Debug
          delegate_to: 127.0.0.1
          debug:
            msg: 'Password: {{ secrets.skolo_password }}'
