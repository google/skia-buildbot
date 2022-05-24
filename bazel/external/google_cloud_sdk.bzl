"""This module defines the google_cloud_sdk repository rule.

Note that this rule is not fully hermetic. See the rule's documentation for details.
"""

load(":common.bzl", "fail_if_nonzero_status")

# Taken from https://cloud.google.com/sdk/docs/downloads-versioned-archives on 2022-05-23.
_GOOGLE_CLOUD_CLI_LINUX_URL = "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-386.0.0-linux-x86_64.tar.gz"
_GOOGLE_CLOUD_CLI_LINUX_SHA256 = "afadfe261e8df24fda780db6fd9be6929df25cf99fd718384eaa7128206349a0"

def _google_cloud_sdk_impl(repository_ctx):
    is_linux = repository_ctx.os.name.lower().startswith("linux")

    if not is_linux:
        # Support for other operating systems can be added as needed.
        fail("OS not yet supported: %s." % repository_ctx.os.name)

    # Download the Google Cloud SDK.
    repository_ctx.download_and_extract(
        _GOOGLE_CLOUD_CLI_LINUX_URL,
        output = "google-cloud-sdk",
        sha256 = _GOOGLE_CLOUD_CLI_LINUX_SHA256,
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
        ],
        quiet = repository_ctx.attr.quiet,
    )
    fail_if_nonzero_status(exec_result, "Failed to install Cloud Emulators.")

google_cloud_sdk = repository_rule(
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
