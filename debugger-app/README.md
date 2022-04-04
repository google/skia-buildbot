Skia Debugger
==========================

A web-based tool for viewing and inspecting SKP and MSKP files (recorded SkPictures)

The debugger consists of one-page app that uses a build of CanvasKit with extra bindings.
The wasm module draws the SKP file being inspected to the canvas in the center of the page.
Source for this wasm module is in the Skia repository at //modules/canvaskit/debugger_bindings.cpp

Running locally
---------------

Run the following to run the debugger instance locally using the version built from ToT.
Talk to the Skia Infra team if you have permission issues that need to be sorted out.

```
make run-local-instance
```

To run debugger locally with a custom build of debugger, run the following in the Skia repo.
```
make -C modules/canvaskit with_debugger
```
This should copy the canvaskit.js, canvaskit.wasm, and canvaskit.d.ts to
`//debugger-app/wasm_libs/local_build`, assuming you have the `SKIA_INFRA_ROOT` environment
variable set. Then, you can run (in this repo)
```
make run-with-custom
```
Do not check in those files that were copied.

The application can now be loaded at http://localhost:8000/

The port can be changed via an environment variable, e.g.
```
DEBUGGER_LOCAL_PORT=:8123 make run-with-custom
```

Production deployment
---------------------

There is a task_driver that will deploy this application at ToT configured in
`infra/bots/task_drivers/push_apps_from_skia_wasm_images/push_apps_from_skia_wasm_images.go`
It runs `make bazel_release_ci` to use the freshly built WASM/JS files to build the container.

In production, the app is served from go/debugger-app/main.go.
See https://skia-review.googlesource.com/c/buildbot/+/334818 for the initial deployment.