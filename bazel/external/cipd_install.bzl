"""This module defines the cipd module extension.

This extension hermetically installs a CIPD package as an external Bazel
repository.

Use `download_http` to download via CIPD's HTTP endpoint. This reduces overhead
when authentication is not required. Use download_cipd when authentication is
required, eg. internal packages.

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
cipd.download_http(
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
    return filepath.Join(bazel.RunfilesDir(), "+cipd+git_amd64_linux/bin/git")
}
```

Note that runfile generation is disabled on Windows by default, and must be enabled with
--enable_runfiles[2] for the above mechanism to work.

[1] https://bazel.build/docs/configurable-attributes
[2] https://bazel.build/reference/command-line-reference#flag--enable_runfiles
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

def _fail_if_nonzero_status(exec_result, msg):
    if exec_result.return_code != 0:
        fail("%s\nExit code: %d\nStdout:\n%s\nStderr:\n%s\n" % (
            msg,
            exec_result.return_code,
            exec_result.stdout,
            exec_result.stderr,
        ))

def _postinstall_script(repository_ctx, script_name, script_content):
    repository_ctx.report_progress("Executing postinstall script...")
    repository_ctx.file(
        script_name,
        content = script_content,
        executable = True,
    )
    exec_result = repository_ctx.execute(
        [repository_ctx.path(script_name)],
        quiet = repository_ctx.attr.quiet,
    )
    _fail_if_nonzero_status(exec_result, "Failed to run postinstall script.")
    repository_ctx.delete(repository_ctx.path(script_name))

_DEFAULT_BUILD_FILE_CONTENT = """
# To add a specific file inside this CIPD package as a dependency, use a label such as
# @my_cipd_pkg//:path/to/file.
# The exclude pattern prevents files with spaces in their names from tripping up Bazel.
exports_files(glob(include=["**/*"], exclude=["**/* *"]))

# Convenience filegroup to add all files in this CIPD package as dependencies.
filegroup(
    name = "all_files",
    # The exclude pattern prevents files with spaces in their names from tripping up Bazel.
    srcs = glob(include=["**/*"], exclude=["**/* *"]),
    visibility = ["//visibility:public"],
)
"""

_CIPD_CLIENT_PLATFORM_TO_DIGEST = {
    "linux-amd64": "1b84a075031772cf05b1a3e0ef2119e96d0492869f971ca72bfdb240ed7cbc45",
    "linux-arm64": "ee6327a8ab86646a664b140d0861147da7d256cbe57df7fa111d1953ccf7b6b4",
    "windows-amd64": "e4567b664ee67eb198fbf44ad10801c57b726dd683da3d37e16b90d403cb00a3",
    "windows-arm64": "ae7068c429aec3e36ebafcb58c5db878af866994c8a9c56b9dbb8e27445c9c7e",
    "mac-amd64": "6bced14c6896592acd2ca5a0675f4523cc4d0f0ec8152df589c7825e5d3a477e",
    "mac-arm64": "96635d202dd2427b1121fca3164abb90cfad91cfae7e114c33dda0e0057b843d",
}

_CIPD_CLIENT_VERSION = "git_revision:32d4a01e34b863fa38d75af646fb7e4492218f08"

def _cipd_install_impl(repository_ctx):
    os = "linux"
    is_windows = False
    is_posix = True
    if "windows" in repository_ctx.os.name.lower():
        is_windows = True
        is_posix = False
        os = "windows"
    elif "mac" in repository_ctx.os.name.lower():
        os = "mac"

    # Download the CIPD client.
    platform = "%s-%s" % (os, repository_ctx.os.arch)
    cipd_digest = _CIPD_CLIENT_PLATFORM_TO_DIGEST[platform]
    repository_ctx.download_and_extract(
        url = _cipd_http_urls("infra/tools/cipd/%s" % platform, _CIPD_CLIENT_VERSION, cipd_digest),
        sha256 = cipd_digest,
        type = "zip",
    )

    # Initialize the CIPD root.
    cipd_client = repository_ctx.path("cipd.exe" if is_windows else "cipd")
    exec_result = repository_ctx.execute(
        [cipd_client, "init", "--force"],
        quiet = repository_ctx.attr.quiet,
    )
    _fail_if_nonzero_status(exec_result, "Failed to initialize CIPD root.")

    # Install the CIPD package.
    repository_ctx.report_progress("Installing CIPD package...")
    exec_result = repository_ctx.execute(
        [
            cipd_client,
            "install",
            repository_ctx.attr.package,
            repository_ctx.attr.version,
            "-root",
            ".",
            "-log-level=debug",
        ],
        quiet = repository_ctx.attr.quiet,
    )
    _fail_if_nonzero_status(exec_result, "Failed to fetch CIPD package.")

    # Generate BUILD.bazel file.
    build_file_content = repository_ctx.attr.build_file_content
    if not build_file_content:
        build_file_content = _DEFAULT_BUILD_FILE_CONTENT
    repository_ctx.file("BUILD.bazel", content = build_file_content)

    # Optionally run the postinstall script if one was given.
    if is_posix and repository_ctx.attr.postinstall_cmds_posix:
        _postinstall_script(
            repository_ctx,
            "postinstall.sh",
            "\n".join(repository_ctx.attr.postinstall_cmds_posix),
        )
    if is_windows and repository_ctx.attr.postinstall_cmds_win:
        _postinstall_script(
            repository_ctx,
            # The .bat extension is needed under Windows, or the OS won't execute the script.
            "postinstall.bat",
            "\n".join(repository_ctx.attr.postinstall_cmds_win),
        )

_cipd_install = repository_rule(
    implementation = _cipd_install_impl,
    attrs = {
        "package": attr.string(
            doc = """CIPD package name, e.g. "infra/3pp/tools/git/linux-amd64".""",
            mandatory = True,
        ),
        "version": attr.string(
            doc = """CIPD package version, e.g. "version:2.29.2.chromium.6".""",
            mandatory = True,
        ),
        "sha256": attr.string(
            doc = "sha256 digest of the package.",
            mandatory = True,
        ),
        "build_file_content": attr.string(
            doc = """If set, will be used as the content of the BUILD.bazel file. Otherwise, a
default BUILD.bazel file will be created with an all_files target.""",
        ),
        "postinstall_cmds_posix": attr.string_list(
            doc = """Post-install commands to execute. Ignored if Bazel is running on a
non-POSIX OS. Optional.""",
        ),
        "postinstall_cmds_win": attr.string_list(
            doc = """Post-install commands to execute. Ignored if Bazel is not running on
Windows. Optional.""",
        ),
        "quiet": attr.bool(
            default = True,
            doc = "Whether stdout and stderr should be printed to the terminal for debugging.",
        ),
    },
    doc = "Hermetically installs a CIPD package as an external Bazel repository.",
)

# Contents of a BUILD file which export all files in all subdirectories.
_all_cipd_files = """
# To add a specific file inside this CIPD package as a dependency, use a label such as
# @my_cipd_pkg//:path/to/file.
# The exclude pattern prevents files with spaces in their names from tripping up Bazel.
exports_files(glob(include=["**/*"], exclude=["**/* *"]))

# Convenience filegroup to add all files in this CIPD package as dependencies.
filegroup(
    name = "all_files",
    # The exclude pattern prevents files with spaces in their names from tripping up Bazel.
    srcs = glob(include=["**/*"], exclude=["**/* *"]),
    visibility = ["//visibility:public"],
)
"""

_export_single_file = """
exports_files(
    ["%s"],
    visibility = ["//visibility:public"]
)
"""

_common_attrs = {
    "name": attr.string(),
    "cipd_package": attr.string(),
    "build_file_content": attr.string(),
    "export_single_file": attr.string(),
    "postinstall_cmds_posix": attr.string_list(),
    "postinstall_cmds_win": attr.string_list(),
    "sha256": attr.string(),
    "tag": attr.string(),
}

_download_http = tag_class(attrs = _common_attrs)

_download_cipd = tag_class(attrs = _common_attrs)

def _get_build_file_content(build_file_content, export_single_file):
    if build_file_content:
        return build_file_content
    elif export_single_file:
        return _export_single_file % export_single_file
    return _all_cipd_files

def _cipd_http_urls(package, tag, sha256):
    cipd_url = "https://chrome-infra-packages.appspot.com/dl/{package}/+/{tag}".format(
        package = package,
        tag = tag,
    )
    mirror_url = "https://cdn.skia.org/bazel/{sha256}.zip".format(sha256 = sha256)
    return [cipd_url, mirror_url]

def _download_package_http(name, cipd_package, tag, sha256, build_file_content = None, export_single_file = None, postinstall_cmds_posix = None, postinstall_cmds_win = None):
    urls = _cipd_http_urls(cipd_package, tag, sha256)
    http_archive(
        name = name,
        build_file_content = _get_build_file_content(build_file_content, export_single_file),
        sha256 = sha256,
        urls = urls,
        patch_cmds = postinstall_cmds_posix,
        patch_cmds_win = postinstall_cmds_win,
        type = "zip",
    )

def _cipd_impl(ctx):
    direct_deps = []
    for mod in ctx.modules:
        for package in mod.tags.download_http:
            _download_package_http(
                name = package.name,
                cipd_package = package.cipd_package,
                tag = package.tag,
                build_file_content = package.build_file_content,
                export_single_file = package.export_single_file,
                postinstall_cmds_posix = package.postinstall_cmds_posix,
                postinstall_cmds_win = package.postinstall_cmds_win,
                sha256 = package.sha256,
            )
            direct_deps.append(package.name)
        for package in mod.tags.download_cipd:
            _cipd_install(
                name = package.name,
                package = package.cipd_package,
                version = package.tag,
                build_file_content = _get_build_file_content(package.build_file_content, package.export_single_file),
                postinstall_cmds_posix = package.postinstall_cmds_posix,
                postinstall_cmds_win = package.postinstall_cmds_win,
                sha256 = package.sha256,
            )
            direct_deps.append(package.name)

    # https://bazel.build/rules/lib/builtins/module_ctx#extension_metadata
    return ctx.extension_metadata(
        # By specifying the direct dependencies, bazel mod tidy will automatically
        # update the use_repo call to add or remove dependencies to the list.
        root_module_direct_deps = direct_deps,
        root_module_direct_dev_deps = [],
        # By setting this line, we are telling Bazel that the generated rules are
        # hermetic all on their own. This *is* the case because we are downloading
        # from CIPD by tag and verifying the sha256 sum.
        # This is a big deal because it means our autorollers can update only the MODULE.bazel
        # file and don't have to update anything in the MODULE.bazel.lock file after.
        reproducible = True,
    )

cipd = module_extension(
    doc = """Bzlmod extension used to download CIPD packages.""",
    implementation = _cipd_impl,
    tag_classes = {
        "download_http": _download_http,
        "download_cipd": _download_cipd,
    },
)

def _cipd_deps_impl(_ctx):
    pass

cipd_deps = module_extension(
    doc = """Install deps needed to download CIPD packages.""",
    implementation = _cipd_deps_impl,
)
