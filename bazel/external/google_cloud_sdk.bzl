"""This module defines the google_cloud_sdk repository rule.

Note that this rule is not fully hermetic. See the rule's documentation for details.
"""

load(":common.bzl", "fail_if_nonzero_status")

def _google_cloud_sdk_impl(repository_ctx):
    # On my x86 Mac running macOS 12, there is no "arch" attr, despite the docs at
    # https://bazel.build/rules/lib/repository_os#arch.
    arch = getattr(repository_ctx.os, "arch", "amd64")

    # URLs taken from https://cloud.google.com/sdk/docs/downloads-versioned-archives on 2022-05-23:
    url = ""
    hash = ""
    if repository_ctx.os.name.lower().startswith("linux"):
        if arch == "amd64":
            url = "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-512.0.0-linux-x86_64.tar.gz"
            hash = "59dab282f72e00b05d8ae47232fdf46765ad6e699cfc70c0a87596e7524dbe68"
    elif repository_ctx.os.name == "mac os x":
        if arch in ("amd64", "x86_64", "aarch64"):
            url = "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-512.0.0-darwin-x86_64.tar.gz"
            hash = "ddf294b7190954803179a5e492a068b50384a4387ddb4f47acd2bc81e0bbf04d"

    if not url:
        # Support for other platforms can be added as needed.
        fail("OS/arch not yet supported: %s/%s." % (repository_ctx.os.name, arch))

    # Download the Google Cloud SDK.
    repository_ctx.download_and_extract(
        url,
        output = "google-cloud-sdk",
        sha256 = hash,
        stripPrefix = "google-cloud-sdk",
    )

    # Generate BUILD.bazel file.
    repository_ctx.file("BUILD.bazel", content = """
filegroup(
    name = "all_files",
    srcs = glob(
        include = ["**/*"],
        # Exclude unnecessary files to prevent RBE errors such as the following:
        #
        #     INVALID_ARGUMENT: Input tree has N files, above the maximum of 70000
        #
        # The exclude patterns below put us well below the maximum of 70000 files:
        #
        #     $ find $(bazel info output_base)/external/google_cloud_sdk \\
        #           | grep -v -E "(__pycache__|\\.pyc$|\\.backup)" \\
        #           | wc -l
        #     23411
        #
        # For reference, this is the number of files without any exclusions:
        #
        #     $ find $(bazel info output_base)/external/google_cloud_sdk | wc -l
        #     63062
        exclude = [
            "**/.backup/**",
            "**/*.pyc",
            "**/*__pycache__",
            "**/platform/gsutil/**",
            "**/surface/ai/**",
            "**/surface/compute/**",
            "**/surface/container/**"
        ],
    ),
    visibility = ["//visibility:public"],
)
""")

    # Install emulators.
    repository_ctx.report_progress("Installing Cloud Emulators...")
    exec_result = repository_ctx.execute(
        [
            "google-cloud-sdk/bin/gcloud",
            "components",
            "install",
            "beta",
            "cloud-firestore-emulator",
            "bigtable",
            "cloud-datastore-emulator",
            "pubsub-emulator",
            "cloud-spanner-emulator",
        ],
        quiet = repository_ctx.attr.quiet,
    )
    fail_if_nonzero_status(exec_result, "Failed to install Cloud Emulators.")

_google_cloud_sdk = repository_rule(
    implementation = _google_cloud_sdk_impl,
    attrs = {
        "quiet": attr.bool(
            default = True,
            doc = "Whether stdout and stderr should be printed to the terminal for debugging.",
        ),
    },
    doc = """Installs the Google Cloud SDK, which provides the gcloud CLI. Non-hermetic.

This rule hermetically downloads the Google Cloud SDK, then installs the Cloud Emulators via the
`gcloud components install ...` command, which is not guaranteed to always download the emulators
at the same exact revision. Therefore, this rule cannot be considered to be fully hermetic.
""",
)

_download = tag_class(attrs = {
    "name": attr.string(mandatory = True),
})

def _gcloud_sdk_impl(ctx):
    for module in ctx.modules:
        for tag in module.tags.download:
            _google_cloud_sdk(name = tag.name)

google_cloud_sdk = module_extension(
    doc = "Bzlmod extension used to download the Google Cloud SDK and emulators.",
    implementation = _gcloud_sdk_impl,
    tag_classes = {
        "download": _download,
    },
)
