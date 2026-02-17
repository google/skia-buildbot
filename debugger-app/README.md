# Skia Debugger

A web-based tool for viewing and inspecting SKP and MSKP files (recorded SkPictures)

The debugger consists of one-page app that uses a build of CanvasKit with extra bindings.
The wasm module draws the SKP file being inspected to the canvas in the center of the page.
Source for this wasm module is in the Skia repository at //modules/canvaskit/debugger_bindings.cpp

## Running locally

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

If there are docker issues, apply a diff like the the following to prevent the build from
pulling a container

```
diff --git a/bazel/skia_app_container.bzl b/bazel/skia_app_container.bzl
index 385443ea0a..8ed383da68 100644
--- a/bazel/skia_app_container.bzl
+++ b/bazel/skia_app_container.bzl
@@ -216,7 +216,9 @@ def skia_app_container(

     oci_image(
         name = image_name,
-        base = base_image,
+        os = "linux",  # "darwin", "windows", etc
+        architecture = "amd64",  #" arm64", "arm", etc
         entrypoint = entrypoint,
         tars = pkg_tars,
         user = default_user,
```

## Production deployment

There are two Louhi flows that work to build this application.
`debugger-app-base` runs `//debugger-app:debugger_container-base` which creates
the application Docker image (minus CanvasKit). `debugger-app` runs
`//infra/debugger-app:debugger_container` (in the Skia repo), which injects
CanvasKit to create the final Docker image. Both of these flows are defined
in
[templates/config.yml](https://louhi-config-internal.googlesource.com/skia-infra/+/refs/heads/master/templates/config.yml).
