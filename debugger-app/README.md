Skia Debugger
==========================

A web-based tool for viewing and inspecting SKP and MSKP files (recorded SkPictures)

The debugger consists of one-page app that that loads a wasm module loosely based on canvaskit
the wasm module draws SKP file being inspected to the canvas in the center of the page.
Source for this wasm module is in the skia repository at //experimental/wasm-skp-debugger


Running locally
---------------

Once, after a clean checkout, run the following

```
cd ../infra-sk
npm ci
cd ../puppeteer-tests/
npm ci
cd ../debugger-app
npm ci
```

Obtain the debugger wasm binary. You can either download the production version with
```
make wasm_libs
```

Or build it locally in the skia repo. (This also copies it to the correct dir)
```
cd experimental/wasm-skp-debugger
make local-debug
```

Start serving the application locally.
(from this dir)

```
make serve
```

The application can now be loaded at http://localhost:8080/dist/main.html

Live reloading is enabled and will catch any change in typescript, javascript, css,
or static files including changes to the wasm binary from `make local-debug` in the
skia repo which copies the .wasm file here.
Note that live reloading will not catch changes to webpack.config.ts

Webpack will show typescript compilation errors in the javascript console of the page
and in the terminal.

Production deployment
---------------------

There is an talk_driver that will deploy this application at ToT configured in
`infra/bots/task_drivers/push_apps_from_skia_wasm_images/push_apps_from_skia_wasm_images.go`
It is running `make release_ci` which in turn runs the `build_release` script in this directory.

In production, the app is served from go/debugger-app/main.go.
See https://skia-review.googlesource.com/c/buildbot/+/334818 for the initial deployment.