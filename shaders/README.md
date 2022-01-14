# Shaders

This is the code for https://shaders.skia.org, a site to allow editing and
running SkSL shaders using WASM in the browser.

To run shaders locally with a custom build of CanvasKit, copy the js and wasm files to
//shaders/wasm_libs/local_build. Then, copy @skia//canvaskit/npm-build/types/index.d.ts to
//shaders/wasm_libs/local_build/canvaskit.d.ts and run:
```
make run-with-custom
```
Do not check in those files you copied. You may also need to copy the canvaskit.d.ts to
//shaders/wasm_libs/types/ in order to appease some IDEs.

## Images

Source images are stored in `gs://skia-public-shader-images` and should be make
world readable so the build of shaders can work from anywhere.
