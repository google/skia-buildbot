"""This module defines the cockroachdb repository rule.

Note that this rule is not fully hermetic. See the rule's documentation for details.
"""

load(":common.bzl", "fail_if_nonzero_status")

def _cockroachdb_cli_impl(repository_ctx):
    url = "https://binaries.cockroachdb.com/cockroach-v21.1.9.linux-amd64.tgz"

    # https://www.cockroachlabs.com/docs/v21.1/install-cockroachdb-linux does not currently
    # provide SHA256 signatures. kjlubick@ downloaded this file and computed this sha256 signature.
    hash = "05293e76dfb6443790117b6c6c05b1152038b49c83bd4345589e15ced8717be3"

    if not repository_ctx.os.name.lower().startswith("linux"):
        # Support for other platforms can be added as needed.
        fail("OS/arch not yet supported: %s." % repository_ctx.os.name)

    # Download the Google Cloud SDK.
    repository_ctx.download_and_extract(
        url,
        output = "cockroachdb",
        sha256 = hash,
        stripPrefix = "cockroach-v21.1.9.linux-amd64",
    )

    # Generate BUILD.bazel file.
    repository_ctx.file("BUILD.bazel", content = """
filegroup(
    name = "cockroachdb",
    srcs = glob(
        include = ["**/*"],
    ),
    visibility = ["//visibility:public"],
)
""")

cockroachdb_cli = repository_rule(
    implementation = _cockroachdb_cli_impl,
    doc = """Installs the cockroachdb cli.""",
)

def _cockroachdb_cli_ext_impl(mctx):
    cockroachdb_cli(
        name = "cockroachdb_cli_ext",
    )

cockroachdb_cli_ext = module_extension(
    implementation = _cockroachdb_cli_ext_impl,
)
