# skottie

A web application for viewing lottie files as rendered by Skia, and going
forward, other renderers. It uses [CanvasKit](https://www.npmjs.com/package/canvaskit-wasm)
to display these.

## Local development

[Install buildbot dependencies](https://github.com/google/skia-buildbot?tab=readme-ov-file#install-dependencies)

Follow instructions to
[enable gcloud](https://cloud.google.com/container-registry/docs/advanced-authentication)

Ask a Skia Maintainer to give view access to GCS buckets
artifacts.skia-public.appspot.com (for downloading build images) and
skottie-renderer (for runtime downloading/uploading of lottie assets).

Run the local server.

```
make run-local-instance
```

By default, you can connect to the web server in your browesr by going to
[localhost:8000](localhost:8000).

If you want see any module changes you make without restarting the whole server,
open another terminal and run:

```
make watch-modules
```

When the changes finish building you can now reload the page to see them.

## Which version of CanvasKit is being used?

When running tests (`bazel test ...`) or a local instance (`make run-local-instance`), the rules
are set up to use a pinned version of CanvasKit. For more information, look in `../WORKSPACE`.

To run skottie locally with a custom build of CanvasKit, copy the files to
//skottie/wasm_libs/local_build and run:

```
make run-with-custom
```

Do not check in those files you copied.

## Deployment

Skottie is made up of the web application, contained within this folder, and
CanvasKit which comes from the Skia repository. The
`//skottie/skottie_container-base` build target creates a "base" Docker image
which contains everything except CanvasKit. This is uploaded to
`gcr.io/skia-public/skottie-base`. The "final" build is in the Skia
repository. That build pulls down the base image, layers CanvasKit on top, and
uploads the final Docker image to `gcr.io/skia-public/skottie-final`.

Both the base and final builds are done in Louhi, and there should be no need
to manually build or upload either Docker image. If this is deemed necessary
then a new base image may be built and uploaded by:

```console
$ make release
```
