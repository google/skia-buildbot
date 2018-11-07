Kubernetes config and applications
==================================

Scripts, YAML files, and utility apps to run our kubernetes cluster(s). Each
cluster will have its own subdirectory that matches the name of the GCE
project.

Ingress
=======

The ingress configs presume that the IP address and certs have already been
created and named, both of which can be done via command line.

Upload certs:

    gcloud compute ssl-certificates create skia-org --certificate=skia.pem --private-key=skia.key

Take care when copying the certs around, for example, download them onto a
ramdrive and unmount the ramdrive after they have been uploaded. See
'create-sa.sh' in this directory.

Reserving a named global IP address:

    gcloud compute addresses create skia-org --global

Configuration
=============

The kubernetes configuration files are kept in a separate repo that will
automaticaly be checked out under /tmp by the pushk command.

Continuous Deployment
=====================

Continuous deployment uses three bits on infrastructure:

  1. The same build_foo config files that are used when building from the desktop.
  2. [GCP Container Builder](https://cloud.google.com/container-builder/).
  3. The continuous-deploy application.

To do continuous deployment for any application that depends upon the Skia
repo, such as fiddler, you will need to add two new steps and a new image
to the `docker/cloudbuild.yaml` file in the Skia repo.

For example:

```
  - name: 'gcr.io/skia-public/infra:prod'
    dir: '/home/skia/golib/src/go.skia.org/infra/fiddlek'
    env:
      - 'ROOT=/workspace/__staging'
      - 'SKIP_BUILD=1'
    args: ['./build_fiddler_release']
    timeout: 600s
```

As a note, the `gcr.io/skia-public/infra:prod` image that this cloud builder step runs on
is re-created on each commit to this repo (see [docker/Dockerfile](../docker/Dockerfile)).

This sets the working directory to the one for the app we want to build, then
runs the `build_fiddler_release` script, but note that we have set the `ROOT`
and `SKIP_BUILD` environment variables so that the script only builds the
application and copies the files into the directory w/o calling docker on that
directory. Also note that we are putting our work product under the /workspace
directory which is preserved between steps by GCP Container Builder.

Also note that we could add a Makefile target that runs all tests and then
runs `build_fiddler_release` and calls make instead of `build_fiddler_release`
directly, which is the preferred method.

Then we add a second step that runs docker on that container to build the
image:

```
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-t', 'gcr.io/$PROJECT_ID/fiddler:$COMMIT_SHA', '/workspace/__staging']
    timeout: 600s
```

See [Substituting Variable Values](https://cloud.google.com/container-builder/docs/configuring-builds/substitute-variable-values)
for more details on `$PROJECT_ID` and `$COMMIT_SHA`.

Finally we add the new image to the list of images that get pushed to
`gcr.io`:

```
images:
  - 'gcr.io/$PROJECT_ID/fiddler:$COMMIT_SHA'
  - 'gcr.io/$PROJECT_ID/skia-release:prod'

```

The continuous-deploy application runs in skia-public and listens for PubSub
messages from GCP Container Builder that is has successfully completed a build
and in that message it includes a list of images it has uploaded. Update the
`continuous-deploy.yaml` file to include the short name of the image you want
continuously deployed as a command-line argument:

```
containers:
  - name: continuous-deploy
    image: gcr.io/skia-public/continuous-deploy:2018-...
    args:
      - "--logtostderr"
      - "--prom_port=:20000"
      - "fiddler"
```

Since continuous-deploy runs `pushk`, all of these deployments will be
recorded in the git repo for skia-public.

Standing up a new cluster in a different project
================================================

  1. Add a new `__skia_NNN` function to `clusters.sh`.
  2. Create the `config-NNN.sh` file.
  3. Copy and modify the `create-cluster-corp.sh` script.
  4. Add a node pool if necessary using the web UI.
  5. Create a new config repo of the form https://skia.googlesource.com/skia-NNNNN-config/.
  6. Add the new cluster to the `clusters` variable in go/pushk/main.go.
