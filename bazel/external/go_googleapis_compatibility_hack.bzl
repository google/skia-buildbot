"""This module defines the go_googleapis_compatibility_hack repository rule."""

def _go_googleapis_compatibility_hack_impl(ctx):
    if ctx.name != "go_googleapis":
        fail("This rule should be named \"go_googleapis\" for this compatibility hack to work.")

    # The visibility of the below aliases should be limited to @com_github_bazelbuild_remote_apis
    # to prevent accidental usages from other dependencies or from our own code.

    # Aliases @go_googleapis//google/api:annotations_go_proto.
    ctx.file("google/api/BUILD.bazel", """
# GENERATED FILE. See //bazel/external/go_googleapis_compatibility_hack.bzl in the Skia Buildbot
# repository.

alias(
  name = "annotations_go_proto",
  actual = "@org_golang_google_genproto//googleapis/api/annotations",
  visibility = ["@com_github_bazelbuild_remote_apis//:__subpackages__"],
)
""")

    # Aliases @go_googleapis//google/longrunning:longrunning_go_proto".
    ctx.file("google/longrunning/BUILD.bazel", """
# GENERATED FILE. See //bazel/external/go_googleapis_compatibility_hack.bzl in the Skia Buildbot
# repository.

alias(
  name = "longrunning_go_proto",
  actual = "@org_golang_google_genproto//googleapis/longrunning",
  visibility = ["@com_github_bazelbuild_remote_apis//:__subpackages__"],
)
""")

    # Aliases @go_googleapis//google/rpc:status_go_pro
    ctx.file("google/rpc/BUILD.bazel", """
# GENERATED FILE. See //bazel/external/go_googleapis_compatibility_hack.bzl in the Skia Buildbot
# repository.

alias(
  name = "status_go_proto",
  actual = "@org_golang_google_genproto//googleapis/rpc/status",
  visibility = ["@com_github_bazelbuild_remote_apis//:__subpackages__"],
)
""")

go_googleapis_compatibility_hack = repository_rule(
    doc = """Hack to make github.com/bazelbuild/remote-apis work with rules_go v0.41.0.

Starting with version v0.41.0, rules_go no longer ships with the @go_googleapis external
repository: https://github.com/bazelbuild/rules_go/releases/tag/v0.41.0. As per the release notes,
Go code imported generated Google API .pb.go files should use the @org_golang_google_genproto
repository instead (https://github.com/googleapis/go-genproto).

Unfortunately, as of 2023-10-30 the https://github.com/bazelbuild/remote-apis Go module is still
distributed with BUILD files that reference the @go_googleapis repository:
https://github.com/bazelbuild/remote-apis/blob/6c32c3b917cc5d3cfee680c03179d7552832bb3f/build/bazel/remote/execution/v2/go/BUILD#L13.

As a compatibility hack, this repository rule provides a fake @go_googleapis repository that
aliases the Bazel labels required by https://github.com/bazelbuild/remote-apis to point to the
correct targets within the @org_golang_google_genproto repository.

We should delete this rule if/when https://github.com/bazelbuild/remote-apis is updated to work
with more recent versions of rules_go.
""",
    implementation = _go_googleapis_compatibility_hack_impl,
)
