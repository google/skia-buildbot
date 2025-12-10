"""Thin wrapper around rbe_exec_properties from the bazel-toolchains repo."""

load("@bazel_toolchains//rules/exec_properties:exec_properties.bzl", _rbe_exec_properties = "rbe_exec_properties")

def _rbe_exec_properties_impl(ctx):
    for module in ctx.modules:
        for tag in module.tags.setup:
            _rbe_exec_properties(name = tag.name)

_setup = tag_class(attrs = {
    "name": attr.string(mandatory = True),
})

rbe_exec_properties = module_extension(
    doc = "Bzlmod extension which thinly wraps rbe_exec_properties from bazel-toolchains.",
    implementation = _rbe_exec_properties_impl,
    tag_classes = {
        "setup": _setup,
    },
)
