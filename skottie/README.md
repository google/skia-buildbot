# skottie

A web application for viewing lottie files as rendered by Skia, and going
forward, other renderers. It uses [CanvasKit](https://www.npmjs.com/package/canvaskit-wasm)
to display these.

## Which version of CanvasKit is being used?

When running tests (`bazel test ...`) or a local instance (`make run-local-instance`), the rules
are set up to get the latest built version of CanvasKit by looking at
`gcr.io/skia-public/skia-wasm-release:prod`. See ./wasm_libs/BUILD.bazel for more.

When deploying, we look in ./build and use the files there. The files checked in to build are just
empty placeholders. The real ones will be provided through our build pipeline.

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
$ make release-base
```
