"""This module provides macros for JS bundles made with the esbuild rule.

To learn more about esbuild and the esbuild Bazel rule, see:
 - https://esbuild.github.io
 - https://docs.aspect.build/rulesets/aspect_rules_esbuild/
"""

load("@aspect_rules_esbuild//esbuild:defs.bzl", "esbuild")

# Global defines required for bundles targeting the browser.
_BROWSER_DEFINES = {
    # Prevent "global is not defined" errors. See https://github.com/evanw/esbuild/issues/73.
    "global": "window",
}

def esbuild_dev_bundle(
        name,
        entry_point,
        output,
        deps = [],
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
        tsconfig = "//:ts_config",
        entry_point = entry_point,
        define = _BROWSER_DEFINES,
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
        output,
        deps = [],
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
        tsconfig = "//:ts_config",
        entry_point = entry_point,
        define = _BROWSER_DEFINES,
        deps = deps,
        minify = True,
        output = output,
        visibility = visibility,
        sourcemap = "",  # Defaults to "linked" (https://esbuild.github.io/api/#sourcemap).
        **kwargs
    )

def esbuild_node_bundle(
        name,
        entry_point,
        output,
        deps = [],
        visibility = ["//visibility:public"],
        **kwargs):
    """Builds a Node.JS JS bundle.

    This macro is a wrapper around the esbuild rule with common settings for Node.JS builds,
    such as sourcemaps, no minification and the --platform esbuild flag set to "node".

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
        tsconfig = "//:ts_config",
        entry_point = entry_point,
        deps = deps,
        sourcemap = "inline",
        sources_content = True,
        output = output,
        platform = "node",
        target = "node16",
        visibility = visibility,
        **kwargs
    )
