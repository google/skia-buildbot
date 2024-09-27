# NOTICE

Usage of Berglas is deprecated in favor of GCP Secret Manager. Scripts
dealing with service account keys have been updated accordingly. Other scripts
still function using Berglas, but Kubernetes Secrets are disallowed in all new
clusters so utility will be limited outside of `skia-public` (`skia-corp` has
been turned down). Any new secrets should live in GCP Secret Manager in the
`skia-infra-public` project. If you are updating existing secrets, consider
migrating them and updating call sites.

# Secrets

Secrets for all clusters are stored encrypted in Google Cloud Storage with the
keys managed via Google KMS. We use the
[berglas](https://github.com/GoogleCloudPlatform/berglas) tool to manage the
encrypted information.

There is a command to copy secrets between cluster directories and a command to
apply all secrets for a cluster, thus making it easy to spin up a new cluster.

All commands move the secrets through stdin/stdout, except `edit-secrets.sh`
which uses a temporary ramdisk to store the files which gets unmounted
immediately after editing. In all cases secrets never sit on the developer's
drive.

## Access control

Files are accessible only by members of skia-root@google.com.

To create local credentials that are usable by berglas, you should run:

        gcloud auth application-default login

Also make sure that the `GOOGLE_APPLICATION_CREDENTIALS` environment variable
isn't set.

## Configuration

The configuration is stored in `config.sh`. If you change the bucket or project,
you will have to re-run `init-berglas.sh`.

## Format

All secrets are stored as base64-encoded kubernetes secrets serialized as YAML.

Secrets are stored in a sub-directory in Google Cloud Storage, the name of which
is the name of the cluster they are used in.

We will use the cluster name `etc` as a special cluster name for secrets that
aren't used in k8s, but are still secrets we need easy access to, such as the
`authorized_keys` file we distribute to all the jumphosts.

## Naming

The GKE cluster names as returned from `kubectl config current-context` aren't
the normal names like `skia-public` that we are used to, so there are helper
functions in `config.sh` to translate between common names and the values
returned from `current-context`.
