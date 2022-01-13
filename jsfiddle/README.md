JS Fiddle
=========

Like fiddle.skia.org, but for JS only libraries. The WASM stuff (PathKit,
CanvasKit) is a primary use case.


Which version of CanvasKit is being used?
-----------------------------------------
When running tests (`bazel test ...`) or a local instance (`make run-local-instance`), the rules
are set up to get the latest built version of CanvasKit and PathKit by looking at
`gcr.io/skia-public/skia-wasm-release:prod`. See ./wasm_libs/BUILD.bazel for more.

When deploying, we look in ./build and use the files there. The files checked in to build are just
empty placeholders. The real ones will be provided through our build pipeline.

To run jsfiddle locally with a custom build of CanvasKit/PathKit, copy the files to
//jsfiddle/wasm_libs/local_build and run:
```
make run-with-custom
```
Do not check in those files you copied.