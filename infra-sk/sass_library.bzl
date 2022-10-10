"""This module provides a wrapper around the sass_library rule from the rules_sass repository."""

# https://github.com/bazelbuild/rules_sass/blob/f6ceac7f5e11424880ae41f9c1a5cfd02968376c/defs.bzl#L1
load("@io_bazel_rules_sass//:defs.bzl", _sass_library = "sass_library")

def sass_library(name, deps = [], visibility = None, **kwargs):
    """Wraps rules_sass's sass_library rule with extra logic to handle NPM dependencies.

    This macro scans the deps argument for any dependencies from NPM packages, groups any such deps
    as a separate sass_library, and adds this library as a dependency of the main target. This
    prevents errors such as:

        in deps attribute of sass_library rule //path/to:my-sass-library: source file
        '@npm//:node_modules/some-npm-package/foo.scss' is misplaced here (expected no files)

    Args:
      name: The name of the target.
      deps: Any sass_library dependencies. This can include .css or .scss files from NPM
        modules, e.g. "npm//:node_modules/some-module/hello.scss".
      visibility: Visibility of the generated sass_library targets.
      **kwargs: Any other arguments to pass to the sass_library rule.
    """

    # If there are any NPM Sass deps, group them as a separate sass_library and add it to
    # the deps argument.
    npm_deps = [dep for dep in deps if dep.startswith("@npm")]
    deps = [dep for dep in deps if dep not in npm_deps]
    if npm_deps != []:
        _sass_library(
            name = name + "_npm_deps",
            srcs = npm_deps,
            visibility = visibility,
        )
        deps.append(name + "_npm_deps")

    _sass_library(
        name = name,
        deps = deps,
        visibility = visibility,
        **kwargs
    )
