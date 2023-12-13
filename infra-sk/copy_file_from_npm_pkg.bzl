"""This module defines the copy_file_from_npm_pkg macro."""

load("@aspect_bazel_lib//lib:copy_file.bzl", "copy_file")
load("@aspect_bazel_lib//lib:directory_path.bzl", "directory_path")

def copy_file_from_npm_pkg(name, npm_package_name, src, out):
    """Makes a local copy of a file found in an NPM package.

    As an example, a local copy of `node_modules/codemirror/lib/codemirror.css` can be created with
    the following rule:

    ```
    copy_file_from_npm_pkg(
        name = "codemirror_css",
        npm_package_name = "codemirror",
        src = "lib/codemirror.css",
        out = "codemirror.css",
    )
    ```

    This macro is necessary because rules_js[1] exposes NPM packages in a way that makes it
    difficult to access individual files directly.

    In https://bazelbuild.slack.com/archives/CEZUUKQ6P/p1661466555661629, a rules_js maintainer
    recommends using the directory_path and copy_file rules from the aspect-build/bazel_lib[2]
    ruleset to create a local copy of files distributed within NPM packages. The copied files will
    appear under the //_bazel_bin directory, and can be depended upon from other rules as if they
    were local source files.

    [1] https://github.com/aspect-build/rules_js
    [2] https://github.com/aspect-build/bazel-lib

    Args:
      name: Name of the rule.
      npm_package_name: Name of the NPM package containing the source file.
      src: Relative path within the NPM package to the source file.
      out: Name of the destination file.
    """

    # Based on
    # https://github.com/aspect-build/rules_js/blob/014b409c6a96d90ee42dbf5274fb15cbc4bbf9f4/examples/js_binary/BUILD.bazel#L400.
    directory_path(
        name = name + "_directory_path",
        directory = "//:node_modules/%s/dir" % npm_package_name,
        path = src,
    )

    copy_file(
        name = name,
        src = name + "_directory_path",
        out = out,
        visibility = ["//visibility:public"],
    )
