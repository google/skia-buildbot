Skia Debugger
==========================

A web-based tool for viewing and inspecting SKP and MSKP files (recorded SkPictures)

The debugger consists of one-page app that that loads a wasm module loosely based on canvaskit
the wasm module draws SKP file being inspected to the canvas in the center of the page.
Source for this wasm module is in the skia repository at //experimental/wasm-skp-debugger

Running locally
---------------

Run the following to run the debugger instance locally using the latest debugger built on ToT.
Talk to the Skia Infra team if you have permission issues that need to be sorted out.

```
make run-local-instance
```

To run debugger locally with a custom build of debugger, copy the debugger.js and debugger.wasm
to //debugger-app/wasm_libs/local_build and run:
```
make run-with-custom
```
Do not check in those files you copied.

The application can now be loaded at http://localhost:8000/

Production deployment
---------------------

There is a task_driver that will deploy this application at ToT configured in
`infra/bots/task_drivers/push_apps_from_skia_wasm_images/push_apps_from_skia_wasm_images.go`
It runs `make bazel_release_ci` to use the freshly built WASM/JS files to build the container.

In production, the app is served from go/debugger-app/main.go.
See https://skia-review.googlesource.com/c/buildbot/+/334818 for the initial deployment.