# Secrets

Secrets for all clusters are stored encrypted in Google Cloud Storage with the
keys managed via Google KMS. We use the
[breglas](https://github.com/GoogleCloudPlatform/berglas) tool
to manage the encrypted information.

## Access control
Files are only accessible by members of skia-root@google.com.

## Configuration
The configuration is stored in `config.sh`. If you change the bucket or project
you will have to re-run `init-berglas.sh`.

## Format

All secrets are stored as base64 encoded kubernetes secrets serialized as YAML.

Secrets are stored in a sub-directory that's the name of the cluster they are
used in.

There is a command to copy secrets between cluster directories, and a command to
apply all secrets for a cluster, thus making it easy to spin up a new cluster.
