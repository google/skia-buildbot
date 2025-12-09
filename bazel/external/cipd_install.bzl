"""This module defines the cipd module extension.

This extension hermetically installs a CIPD package as an external Bazel
repository.

Files in the CIPD package can be added as dependencies to other Bazel targets in two ways: either
individually via a label such as "@my_cipd_pkg//:path/to/file", or by adding
"@my_cipd_pkg//:all_files" as a dependency, which is a filegroup that includes the entire contents
of the CIPD package. The contents of the generated BUILD.bazel file which facilitates this are
configurable, e.g. multiple smaller packages.

Note: Any files with spaces in their names cannot be used by Bazel and are thus excluded
from the generated Bazel rules.

If a Bazel target adds a CIPD package as a dependency, its contents will appear under the runfiles
directory. Example:

```
# MODULE.bazel
cipd.package(
    name = "git_amd64_linux",
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
    tag = "version:2.29.2.chromium.6",
)

# BUILD.bazel
go_library(
    name = "git_util.go",
    srcs = ["git_util.go"],
    data = ["@git_amd64_linux//:all_files"],
    ...
)

# git_util.go
import (
    "path/filepath"

    "go.skia.org/infra/bazel/go/bazel"
)

func FindGitBinary() string {
    return filepath.Join(bazel.RunfilesDir(), "external/_main~cipd~git_amd64_linux/bin/git")
}
```

Note that runfile generation is disabled on Windows by default, and must be enabled with
--enable_runfiles[2] for the above mechanism to work.

[1] https://bazel.build/docs/configurable-attributes
[2] https://bazel.build/reference/command-line-reference#flag--enable_runfiles
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Contents of a BUILD file which export all files in all subdirectories.
_all_cipd_files = """
filegroup(
  name = "all_files",
  # The exclude pattern prevents files with spaces in their names from tripping up Bazel.
  srcs = glob(include=["**/*"], exclude=["**/* *"]),
  visibility = ["//visibility:public"],
)"""

_package = tag_class(attrs = {
    "name": attr.string(),
    "cipd_package": attr.string(),
    "build_file_content": attr.string(),
    "postinstall_cmds_posix": attr.string_list(),
    "postinstall_cmds_win": attr.string_list(),
    "sha256": attr.string(),
    "tag": attr.string(),
})

def _cipd_impl(ctx):
    for mod in ctx.modules:
        for package in mod.tags.package:
            cipd_url = "https://chrome-infra-packages.appspot.com/dl/{package}/+/{tag}".format(
                package = package.cipd_package,
                tag = package.tag,
            )
            mirror_url = "https://cdn.skia.org/bazel/{sha256}.zip".format(sha256 = package.sha256)
            http_archive(
                name = package.name,
                build_file_content = package.build_file_content or _all_cipd_files,
                sha256 = package.sha256,
                urls = [
                    cipd_url,
                    mirror_url,
                ],
                patch_cmds = package.postinstall_cmds_posix,
                patch_cmds_win = package.postinstall_cmds_win,
                type = "zip",
            )

cipd = module_extension(
    doc = """Bzlmod extension used to download CIPD packages.""",
    implementation = _cipd_impl,
    tag_classes = {
        "package": _package,
    },
)
