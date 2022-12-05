"""This module defines the cipd_install repository rule.

The cipd_install repository rule hermetically installs a CIPD package as an external Bazel
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
# WORKSPACE
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
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
    return filepath.Join(bazel.RunfilesDir(), "external/git_amd64_linux/bin/git")
}
```

Note that runfile generation is disabled on Windows by default, and must be enabled with
--enable_runfiles[2] for the above mechanism to work.

[1] https://bazel.build/docs/configurable-attributes
[2] https://bazel.build/reference/command-line-reference#flag--enable_runfiles
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

def cipd_install(
        name,
        build_file_content,
        cipd_package,
        sha256,
        tag,
        postinstall_cmds_posix = None,
        postinstall_cmds_win = None):
    """Download and extract the zipped archive from CIPD, making it available for Bazel rules.

    This is a wrapper around the built-in http_archive rule.

    Args:
        name: The name of the Bazel "repository" created. For example, if name is "alpha_beta",
              the full Bazel label will start with @alpha_beta//
        build_file_content: CIPD packages do not come with BUILD.bazel files, so we must supply
                            one. This should generally contain exports_files or filegroup.
                            See also all_cipd_files() and export_cipd_files for helpers to create
                            these contents.
        cipd_package: The full name of the CIPD package. This is a "path" from the root of CIPD.
                      This should be a publicly accessible package, as authentication is not
                      supported.
        sha256: The sha256 hash of the zip archive downloaded from CIPD. This should match the
                official CIPD website.
        tag: Represents the version of the CIPD package to download.
             For example, git_revision:abc123...
        postinstall_cmds_posix: Optional Bash commands to run on Mac/Linux after download.
        postinstall_cmds_win: Optional Powershell commands to run on Windows after download.
    """
    cipd_url = "https://chrome-infra-packages.appspot.com/dl/"
    cipd_url += cipd_package
    cipd_url += "/+/"
    cipd_url += tag

    mirror_url = "https://storage.googleapis.com/skia-world-readable/bazel/"
    mirror_url += sha256
    mirror_url += ".zip"

    # https://bazel.build/rules/lib/repo/http#http_archive
    http_archive(
        name = name,
        build_file_content = build_file_content,
        sha256 = sha256,
        urls = [
            cipd_url,
            mirror_url,
        ],
        patch_cmds = postinstall_cmds_posix,
        patch_cmds_win = postinstall_cmds_win,
        type = "zip",
    )

def all_cipd_files():
    """Returns the contents of a BUILD file which export all files in all subdirectories."""
    return """
filegroup(
  name = "all_files",
  # The exclude pattern prevents files with spaces in their names from tripping up Bazel.
  srcs = glob(include=["**/*"], exclude=["**/* *"]),
  visibility = ["//visibility:public"],
)"""

def export_cipd_files(list_of_files):
    """Returns the contents of a BUILD file which exports only the given files.

    Args:
        list_of_files: list of strings containing paths relative to the root of the extracted files.
    Returns:
        A string containing a public export_files rule.
    """
    contents = """
exports_files(
    ["""
    for file in list_of_files:
        contents += '"' + file + '",'

    contents += """],
    visibility = ["//visibility:public"]
)
"""
    return contents
