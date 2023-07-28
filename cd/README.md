The `cd` image produced in this package encapsulates the `build-images`
executable (`./go/build-images`) which is used in numerous Louhi stages.

These stages are configured in https://louhi-config-internal.googlesource.com/skia-infra/+/refs/heads/master/.louhi/stage-types/
and used to do things like "Build a Docker container using the Bazel target //foo:bar".

## Deployment

Because `cd` is used by Louhi it has no Louhi trigger and must be manually
triggered. This is done by:

1. Navigate to the
   [cd Louhi page](https://louhi.dev/6316342352543744/flow-detail/c1ffa520-35c8-47d4-843d-c94c1b8864d3?branch=main).
2. Press the "Run" button to display the "New execution" dialog.
3. Press the "RUN" button. Do not add any stage parameters.

When the `cd` flow has finished building a new Docker image will be listed
at https://louhi.dev/6316342352543744/artifacts. This must manually be
integrated into the skia-infra repo by updating all `cd` digest references to
the latest digest. Example CL: http://go/louhicl/100543.

Keep an eye on Louhi flow builds to ensure new builds are passing.
