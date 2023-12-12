"""This module defines the copy_file_from_npm_pkg macro."""

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

    This macro exists to ease the transition from rules_nodejs[1] to rules_js[2], see b/314813928.
    The rules_js ruleset exposes NPM packages in a way that makes it difficult to access individual
    files. As discussed in https://bazelbuild.slack.com/archives/CEZUUKQ6P/p1661466555661629, the
    recommended way to access individual files from NPM packages under rules_js is by creating
    local copies of those file using helper rules from the aspect-build/bazel_lib[3] ruleset.

    Right now this macro is just a simple genrule that copies files using the shell's "cp" command,
    but we will replace it with the aforementioned rules as part of the migration from rules_nodejs
    to rules_js.

    [1] https://github.com/bazelbuild/rules_nodejs
    [2] https://github.com/aspect-build/rules_js
    [3] https://github.com/aspect-build/bazel-lib

    Args:
      name: Name of the rule.
      npm_package_name: Name of the NPM package containing the source file.
      src: Relative path within the NPM package to the source file.
      out: Name of the destination file.
    """

    native.genrule(
        name = name,
        srcs = ["@npm//:node_modules/%s/%s" % (npm_package_name, src)],
        outs = [out],
        cmd = "cp $< $@",
    )
