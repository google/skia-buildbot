# Shaders

This is the code for https://shaders.skia.org, a site to allow editing and
running SkSL shaders using WASM in the browser.

## Google Cloud Storage and CORS

By default you can't get the pixels that make up an image if that image is
loaded cross-origin. Since our source images come from GCS we need to make a lot
of changes, including changing our CSP to allow loading images from the GCS
domain, tagging any `src` tags as crossorigin, and then setting a CORS
configuration for the bucket where we store the images.

```
$ ./set-gce-cors-rules.sh
```
