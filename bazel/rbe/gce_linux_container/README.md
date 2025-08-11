This file contains the image that is put onto the Remote Build Execution (RBE) machines (also known
as executors) when running remote execution steps for this infra repo. Skia has its own
[version of this image](https://github.com/google/skia/blob/cedfe6ee4a77f59955475326d83543b697c0fae8/bazel/rbe/gce_linux_container/Dockerfile).

It's meant to be a lightweight image, as the Bazel rules should bring in their own toolchains
for most things.

To iterate locally, run the `build.sh` script in this directory (it will fail to push the image
at the end, because write permissions are only available on demand to developers). Once the image
appears to work, commit it and then invoke the
["Build infra-rbe-linux" Louhi flow](https://louhi.corp.goog/6316342352543744/flow-detail/97084790-2e32-4756-98a3-55d860f2530b?branch=main)
manually. This will produce an image and then following the steps in `../generated` can incorporate
that new image.
