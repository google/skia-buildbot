"""This module provides macros for JS bundles made with the esbuild rule.

To learn more about esbuild and the esbuild Bazel rule, see:
 - https://esbuild.github.io
 - https://bazelbuild.github.io/rules_nodejs/esbuild.html
 - https://github.com/bazelbuild/rules_nodejs/blob/stable/packages/esbuild/esbuild.bzl
"""

load("@npm//@bazel/esbuild:index.bzl", "esbuild")

def esbuild_dev_bundle(
        name,
        entry_point,
        deps = [],
        output = None,
        visibility = ["//visibility:public"],
        **kwargs):
    """Builds a development JS bundle.

    This macro is a wrapper around the esbuild rule with common settings for development builds,
    such as sourcemaps and no minification.

    Args:
      name: The name of the rule.
      entry_point: Entry-point TypeScript or JavaScript file.
      deps: Any ts_library dependencies.
      output: Name of the output JS file.
      visibility: Visibility of the rule.
      **kwargs: Any other arguments to be passed to the esbuild rule.
    """
    esbuild(
        name = name,
        config = "//infra-sk:esbuild_config",
        entry_point = entry_point,
        deps = deps,
        sourcemap = "inline",
        sources_content = True,
        output = output,
        visibility = visibility,
        **kwargs
    )

def esbuild_prod_bundle(
        name,
        entry_point,
        deps = [],
        output = None,
        visibility = ["//visibility:public"],
        **kwargs):
    """Builds a production JS bundle.

    This macro is a wrapper around the esbuild rule with common settings for production builds, such
    as minifying the output.

    Args:
      name: The name of the rule.
      entry_point: Entry-point TypeScript or JavaScript file.
      deps: Any ts_library dependencies.
      output: Name of the output JS file.
      visibility: Visibility of the rule.
      **kwargs: Any other arguments to be passed to the esbuild rule.
    """
    esbuild(
        name = name,
        config = "//infra-sk:esbuild_config",
        entry_point = entry_point,
        deps = deps,
        output = output,
        args = {"sourcemap": False},  # Tell the esbuild binary not to produce a sourcemap.
        # By default, the esbuild rule generates a sourcemap as a .js.map file alongside the .js
        # bundle. The only way to prevent this behavior is to ask the rule for an inline sourcemap
        # (i.e. a comment inside the .js bundle), and then tell the actual esbuild binary not to
        # produce a sourcemap. This is probably a bug in the esbuild rule.
        sourcemap = "inline",
        minify = True,
        visibility = visibility,
        **kwargs
    )
