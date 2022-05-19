"""This module defines the cipd_install repository rule.

The cipd_install repository rule hermetically installs a CIPD package as an external Bazel
repository.

Files in the CIPD package can be added as dependencies to other Bazel targets in two ways: either
individually via a label such as "@my_cipd_pkg//:path/to/file", or by adding
"@my_cipd_pkg//:all_files" as a dependency, which is a filegroup that includes the entire contents
of the CIPD package.

If a Bazel target adds a CIPD package as a dependency, its contents will appear under the runfiles
directory. Example:

```
# WORKSPACE
cipd_install(
    name = "git_linux",
    package = "infra/3pp/tools/git/linux-amd64",
    version = "version:2.29.2.chromium.6",
)

# BUILD.bazel
go_library(
    name = "git_util.go",
    srcs = ["git_util.go"],
    data = ["@git_linux//:all_files"],
    ...
)

# git_util.go
import (
    "path/filepath"

    "go.skia.org/infra/bazel/go/bazel"
)

func FindGitBinary() string {
    return filepath.Join(bazel.RunfilesDir(), "external/git_linux/bin/git")
}
```

For Bazel targets that must support multiple operating systems, one can declare OS-specific CIPD
packages in the WORKSPACE file, and select the correct package according to the host OS via a
select[1] statement. Example:

```
# WORKSPACE
cipd_install(
    name = "git_linux",
    package = "infra/3pp/tools/git/linux-amd64",
    version = "version:2.29.2.chromium.6",
)
cipd_install(
    name = "git_win",
    package = "infra/3pp/tools/git/windows-amd64",
    version = "version:2.29.2.chromium.6",
)

# BUILD.bazel
go_library(
    name = "git_util.go",
    srcs = ["git_util.go"],
    data = select({
        "@platforms//os:linux": ["@git_linux//:all_files"],
        "@platforms//os:windows": ["@git_win//:all_files"],
    }),
    ...
)
```

As an alternative, we could extract any such select statements as Bazel macros, which would keep
BUILD files short. Example:

```
# cipd_packages.bzl
def git():
    return select({
        "@platforms//os:linux": ["@git_linux//:all_files"],
        "@platforms//os:windows": ["@git_win//:all_files"],
    })

# BUILD.bazel
load(":cipd_packages.bzl", "git")

go_library(
    name = "git_util.go",
    srcs = ["git_util.go"],
    data = git(),
    ...
)
```

Note that runfile generation is disabled on Windows by default, and must be enabled with
--enable_runfiles[2] for the above mechanism to work.

[1] https://bazel.build/docs/configurable-attributes
[2] https://bazel.build/reference/command-line-reference#flag--enable_runfiles


"""

def _fail_if_nonzero_status(exec_result, msg):
    if exec_result.return_code != 0:
        fail("%s\nExit code: %d\nStdout:\n%s\nStderr:\n%s\n" % (
            msg,
            exec_result.return_code,
            exec_result.stdout,
            exec_result.stderr,
        ))

def _cipd_install_impl(repository_ctx):
    is_windows = "windows" in repository_ctx.os.name.lower()

    # Install the CIPD package.
    cipd_client = Label("@depot_tools//:cipd.bat" if is_windows else "@depot_tools//:cipd")
    repository_ctx.report_progress("Installing CIPD package...")
    exec_result = repository_ctx.execute(
        [
            repository_ctx.path(cipd_client),
            "install",
            repository_ctx.attr.package,
            repository_ctx.attr.version,
            "-root",
            ".",
        ],
        quiet = repository_ctx.attr.quiet,
    )
    _fail_if_nonzero_status(exec_result, "Failed to fetch CIPD package.")

    # Generate BUILD.bazel file.
    repository_ctx.file("BUILD.bazel", content = """
# To add a specific file inside this CIPD package as a dependency, use a label such as
# @my_cipd_pkg//:path/to/file.
exports_files(glob(["**/*"]))

# Convenience filegroup to add all files in this CIPD package as dependencies.
filegroup(
    name = "all_files",
    srcs = glob(["**/*"]),
    visibility = ["//visibility:public"],
)
""")

    # Optionally run the postinstall script if one was given.
    if repository_ctx.attr.postinstall_script != "":
        # The .bat extension is needed under Windows, or the OS won't execute the script.
        script_name = "postinstall.bat" if is_windows else "postinstall.sh"
        repository_ctx.report_progress("Executing postinstall script...")
        repository_ctx.file(
            script_name,
            content = repository_ctx.attr.postinstall_script,
            executable = True,
        )
        exec_result = repository_ctx.execute(
            [repository_ctx.path(script_name)],
            quiet = repository_ctx.attr.quiet,
        )
        _fail_if_nonzero_status(exec_result, "Failed to run postinstall script.")
        repository_ctx.delete(repository_ctx.path(script_name))

cipd_install = repository_rule(
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
        "postinstall_script": attr.string(
            doc = """Contents of an optional script to execute after installing the package.""",
        ),
        "quiet": attr.bool(
            default = True,
            doc = "Whether stdout and stderr should be printed to the terminal for debugging.",
        ),
    },
    doc = "Hermetically installs a CIPD package as an external Bazel repository.",
)
