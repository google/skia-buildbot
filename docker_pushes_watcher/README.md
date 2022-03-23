Docker Pushes Watcher
=====================

Docker pushes watcher is an app that with new task drivers and new jobs has replaced
Google Cloud Build for Skia. The tracking bug for this work was [skbug/9514](https://bugs.chromium.org/p/skia/issues/detail?id=9514).


## Previous Cloud Build framework

Skia used to trigger builds on Google Cloud's [Cloud Build](https://cloud.google.com/cloud-build/)
framework using this [cloudbuild.yaml](https://skia.googlesource.com/skia/+show/6f217e0f8d2e5f06e36d426becd818aeefe39919/docker/cloudbuild.yaml) file.

Using cloud build, the "skia-release" and "skia-wasm-release" images were created and pushed per
commit in the Skia repository. The "infra" image was created and pushed per commit in the Buildbot
 repository.

Additionally [cloudbuild.yaml](https://skia.googlesource.com/skia/+show/6f217e0f8d2e5f06e36d426becd818aeefe39919/docker/cloudbuild.yaml)
created and pushed new images for various apps like fiddler, skottie, particles, debugger, etc.

The [continuous-deploy](https://skia.googlesource.com/buildbot/+show/1985cd594e9f8c7bdec82b89e110df7466ee3cf8/kube/go/continuous-deploy/)
app then ran pushk on those apps and deployed them to k8s.


## Replacement framework with Task Drivers and Docker Pushes Watcher app

The Cloud build framework worked but did not use Skia infra framework and was thus difficult to see
failures and diagnose problems. The framework was replaced with task drivers and the
docker-pushes-watcher app.

Now all failures when building apps cause the corresponding jobs to turn red on status.skia.org for
the commits that break things, this makes failures much easier to find for the Skia Gardener.
Failures in the docker-pushes-watcher app show up as alerts for the Infra Gardener to diagnose and fix.


### Task Drivers

A task driver was written to build and push a specified docker image
([build_push_docker_image](https://skia.googlesource.com/buildbot/+show/master/infra/bots/task_drivers/build_push_docker_image/)).

The following jobs were then created in the Skia repository:
* [Housekeeper-PerCommit-CreateDockerImage_Skia_Release](https://status.skia.org/repo/skia?commit_label=author&filter=search&search_value=CreateDockerImage_Skia_Release)
  job to create and push the "gcr.io/skia-public/skia-release" image using this [Dockerfile](https://skia.googlesource.com/skia/+show/master/docker/skia-release/Dockerfile).
* [Housekeeper-PerCommit-CreateDockerImage_Skia_WASM_Release](https://status.skia.org/repo/skia?commit_label=author&filter=search&search_value=CreateDockerImage_Skia_WASM_Release)
  job to create and push the "gcr.io/skia-public/skia-wasm-release" image using this [Dockerfile](https://skia.googlesource.com/skia/+show/master/docker/skia-wasm-release/Dockerfile).

These jobs could run out of order because of backfilling. Due to this the "Docker Pushes Watcher"
app (described below) calculates which image is the most recent and then tags it with the
"prod" tag.

Task Drivers were also written to create and push Docker images of various apps that depend on the
"gcr.io/skia-public/skia-release" and "gcr.io/skia-public/skia-wasm-release" images:
* [push_apps_from_skia_image](https://skia.googlesource.com/skia/+show/master/infra/bots/task_drivers/push_apps_from_skia_image/)
* [push_bazel_apps_from_wasm_image](https://skia.googlesource.com/skia/+show/master/infra/bots/task_drivers/push_bazel_apps_from_wasm_image/)

The following jobs were created in the Skia repo using the above task drivers:
* [Housekeeper-PerCommit-PushAppsFromSkiaDockerImage](https://status.skia.org/repo/skia?commit_label=author&filter=search&search_value=PushAppsFromSkiaDockerImage)
  job to create and push docker images for fiddler and api apps.
* [Housekeeper-PerCommit-PushBazelAppsFromWASMDockerImage](https://status.skia.org/repo/skia?commit_label=author&filter=search&search_value=PushBazelAppsFromWASMDockerImage)
  job to create and push docker images for jsfiddle, skottie, debugger, and particle apps.

All above task drivers send a [pubsub message](https://skia.googlesource.com/buildbot/+show/master/go/docker/build/pubsub/pubsub.go#15)
when a docker image is created and pushed.

### Docker Pushes Watcher App

The [docker pushes watcher](https://skia.googlesource.com/buildbot/+show/master/docker_pushes_watcher/)
app listens for [pubsub messages](https://skia.googlesource.com/buildbot/+show/master/go/docker/build/pubsub/pubsub.go#15) for 2 main tasks:
* Tags images in the app's list with the "prod" tag when they correspond to the latest commit in the
  Skia/Buildbot repository. This is done to account for jobs running out of order because of
  backfilling.
* Deploys apps to k8s using pushk for images in the app's list when they correspond to the
  latest commit.
