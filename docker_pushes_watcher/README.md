Docker Pushes Watcher
=====================

Docker pushes watcher is an app that along side a few task drivers has replaced
Google Cloud Build for Skia ([skbug/9514](https://bugs.chromium.org/p/skia/issues/detail?id=9514)).


## Previous Cloud Build framework

Skia used to use this [cloudbuild.yaml](https://skia.googlesource.com/skia/+/6f217e0f8d2e5f06e36d426becd818aeefe39919/docker/cloudbuild.yaml)
file to trigger builds on Google Cloud's [Cloud Build](https://cloud.google.com/cloud-build/) framework.

"skia-release" and "skia-wasm-release" images were created and pushed per commit in the Skia repository.
"infra" image was created and pushed per commit in the Buildbot repository.
The [cloudbuild.yaml](https://skia.googlesource.com/skia/+/6f217e0f8d2e5f06e36d426becd818aeefe39919/docker/cloudbuild.yaml) caused new
images to be created for various apps like fiddler, skottie, particles, debugger, etc.

The [continuous-deploy](https://skia.googlesource.com/buildbot/+/master/kube/go/continuous-deploy/) app used to then run pushk on those
apps deploying them to k8s.


## Replacement framework with Task Drivers and Docker Pushes Watcher app

The Cloud build framework worked but did not use Skia infra framework and was thus difficult to see
failures and diagnose problems. The framework was replaced with task drivers and the Docker pushes
watcher app.

### Task Drivers

A task driver was written to build and push a specified docker image ([build_push_docker_image](https://skia.googlesource.com/buildbot/+/master/infra/bots/task_drivers/build_push_docker_image/)).
The following bots were then created in the different repositories:
* [Infra-PerCommit-CreateDockerImage](https://status.skia.org/repo/infra?commit_label=author&filter=search&search_value=CreateDockerImage) bot to create and push the "gcr.io/skia-public/infra" image using this [Dockerfile](https://skia.googlesource.com/buildbot/+/master/docker/Dockerfile) in the Buildbot repository.
* []() bot to create and push the "gcr.io/skia-public/skia-release" image using this [Dockerfile]() in the Skia repository.
* []() bot to create and push the "gcr.io/skia-public/skia-wasm-release" image using this [Dockerfile]() in the Skia repository.

These bots could run out of order because of backfilling. Due to this the "Docker Pushes Watcher" app determines which image is the
most recent and then tags it with the "prod" tag.


Task Drivers were also written to

