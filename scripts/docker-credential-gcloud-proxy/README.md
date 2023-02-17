# docker-credential-gcloud-proxy.py

This is a wrapper script around the `docker-credential-gcloud` program, which is installed by the
[Google Cloud SDK](https://cloud.google.com/sdk). It pipes through its stdin and command-line
arguments to `docker-credential-gcloud`, logs `docker-credential-gcloud`'s arguments, stdin,
stdout, stderr and exit code to a file on disk, and emits the stdout, stderr and exit code back to
the parent process.

The purpose of this script is to debug Docker authorization issues on GCE machines.

## Usage

This script replaces `/usr/bin/docker-credential-gcloud`, which is a symlink to
`/usr/lib/google-cloud-sdk/bin/docker-credential-gcloud`.

First, install this script on a machine with `install.sh`, e.g.:

```
$ install.sh skia-e-gce-234
```

The `install.sh` script will delete the `/usr/bin/docker-credential-gcloud` symlink and copy
`docker-credential-gcloud-proxy.py` as `/usr/bin/docker-credential-gcloud` on the machine.

Then, wait for a Docker task to run on that machine. Any interactions with
`docker-credential-gcloud` will be logged in `/docker-credential-gcloud-proxy.log`. Logs for
multiple invocations will be appended to said file.

Once you're finished, use `uninstall.sh` to restore the original `docker-credential-gcloud` program
and delete the log file, e.g.:

```
$ uninstall.sh skia-e-gce-234
```

This will delete `/docker-credential-gcloud-proxy.log` and `/usr/bin/docker-credential-gcloud`, and
will restore the latter as a symlink to `/usr/lib/google-cloud-sdk/bin/docker-credential-gcloud`.

## How Docker authorization works

The `docker` command reads file `$HOME/.docker/config.json` to get the credentials needed to
interact with container registries.
[Google Container Registry](https://cloud.google.com/container-registry) users typically run the
`gcloud auth configure-docker` command to get their credentials, which populates
`$HOME/.docker/config.json` with the following contents:

```
{
  "credHelpers": {
    "gcr.io": "gcloud",
    "us.gcr.io": "gcloud",
    "eu.gcr.io": "gcloud",
    "asia.gcr.io": "gcloud",
    "staging-k8s.gcr.io": "gcloud",
    "marketplace.gcr.io": "gcloud"
  }
}
```

[Credential helpers](https://docs.docker.com/engine/reference/commandline/login/#credential-helpers)
are programs that can provide credentials for specific registries. When a user tries to e.g. pull
an image from a registry with the `docker` command, `docker` will look for a "credHelpers"
key/value pair where the key corresponds to the registry, and it will invoke a credential helper
named `docker-credential-<value>` to get credentials for that registry. For example, for an entry
such as `"gcr.io": "gcloud"` in the above `config.json` file, `docker` will invoke a credential
helper named `docker-credential-gcloud`.

Credential helpers follow a simple
[protocol](https://docs.docker.com/engine/reference/commandline/login/#credential-helper-protocol).
They take a command-line argument to identify the action (either `store`, `get` or `erase`), and
they take a payload via stdin. The `get` action takes a string payload with the server address that
docker needs credentials for, e.g.:

```
$ echo gcr.io | docker-credential-gcloud get
{
  "Secret": "...",
  "Username": "_dcgcloud_token"
}
```