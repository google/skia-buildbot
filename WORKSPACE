workspace(
    name = "skia_infra",
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")
load("//bazel:gcs_mirror.bzl", "gcs_mirror_url")

# Read the instructions in //bazel/rbe/generated/README.md before updating this repository.
#
# We load bazel-toolchains here, rather than closer where it's first used (RBE container toolchain),
# because the grpc_deps() macro (invoked below) will pull an old version of bazel-toolchains if it's
# not already defined.
http_archive(
    name = "bazel_toolchains",
    sha256 = "179ec02f809e86abf56356d8898c8bd74069f1bd7c56044050c2cd3d79d0e024",
    strip_prefix = "bazel-toolchains-4.1.0",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-toolchains/releases/download/4.1.0/bazel-toolchains-4.1.0.tar.gz",
        "https://github.com/bazelbuild/bazel-toolchains/releases/download/4.1.0/bazel-toolchains-4.1.0.tar.gz",
    ],
)

##################
# Miscellaneous. #
##################

load("@bazel_toolchains//rules/exec_properties:exec_properties.bzl", "rbe_exec_properties")

# Defines a local repository named "exec_properties" which defines constants such as NETWORK_ON.
# These constants are used by the //:rbe_custom_platform build rule.
#
# See https://github.com/bazelbuild/bazel-toolchains/tree/master/rules/exec_properties.
rbe_exec_properties(
    name = "exec_properties",
)

##################
# CIPD packages. #
##################

load("//bazel/external:cipd_install.bzl", "all_cipd_files", "cipd_install")

cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
    tag = "version:2.29.2.chromium.6",
)

cipd_install(
    name = "git_amd64_windows",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/windows-amd64",
    postinstall_cmds_win = [
        "mkdir etc",
        "bin/git.exe config --system user.name \"Bazel Test User\"",
        "bin/git.exe config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/windows-amd64/+/version:2.29.2.chromium.6
    sha256 = "9caaf2c6066bdcfa94f917323c4031cf7e32572848f8621ecd0d328babee220a",
    tag = "version:2.29.2.chromium.6",
)

cipd_install(
    name = "vpython_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/tools/luci/vpython/linux-amd64",
    # From https://chrome-infra-packages.appspot.com/p/infra/tools/luci/vpython/linux-amd64/+/git_revision:31868238187077557113efa2bd4e2c7e3b3ec970
    sha256 = "ec210b3873665208c42e80883546d22d5f448f04e736f1e1fc015da7fc3003a3",
    tag = "git_revision:31868238187077557113efa2bd4e2c7e3b3ec970",
)

cipd_install(
    name = "cpython3_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/cpython3/linux-amd64",
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/cpython3/linux-amd64/+/version:2@3.11.7.chromium.31
    sha256 = "0ff2955adf65e2921c4abd8e2848862d3c7731feeda5c506f44e796aa4af2dc7",
    tag = "version:2@3.11.7.chromium.31",
)

cipd_install(
    name = "patch_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "skia/bots/patch_linux_amd64",
    # From https://chrome-infra-packages.appspot.com/p/skia/bots/patch/+/version:0
    sha256 = "757fd36db06f291f77a91aa314b855af449665a606d627ce16c36813464e1df6",
    tag = "version:0",
)

#############################################################
# Google Cloud SDK (needed for the Google Cloud Emulators). #
#############################################################

load("//bazel/external:google_cloud_sdk.bzl", "google_cloud_sdk")

google_cloud_sdk(name = "google_cloud_sdk")

##################################################
# CockroachDB (used as an "emulator" for tests). #
##################################################

http_archive(
    name = "cockroachdb_linux",
    build_file_content = """
filegroup(
    name = "all_files",
    srcs = glob(["**/*"]),
    visibility = ["//visibility:public"]
)
""",
    # https://www.cockroachlabs.com/docs/v21.1/install-cockroachdb-linux does not currently
    # provide SHA256 signatures. kjlubick@ downloaded this file and computed this sha256 signature.
    sha256 = "05293e76dfb6443790117b6c6c05b1152038b49c83bd4345589e15ced8717be3",
    strip_prefix = "cockroach-v21.1.9.linux-amd64",
    urls = gcs_mirror_url(
        sha256 = "05293e76dfb6443790117b6c6c05b1152038b49c83bd4345589e15ced8717be3",
        url = "https://binaries.cockroachdb.com/cockroach-v21.1.9.linux-amd64.tgz",
    ),
)

##################################################
# PGAdaptor #
##################################################
http_archive(
    name = "pgadapter",
    build_file_content = """
filegroup(
    name = "all_files",
    srcs = glob(["**/*"]),
    visibility = ["//visibility:public"]
)
""",
    sha256 = "2dbb655dddc113eb14659e121839a9d3a5de34544ce88a1dd4dd88c23d436ae3",
    urls = ["https://storage.googleapis.com/pgadapter-jar-releases/pgadapter-v0.47.1.tar.gz"],
)

#################################################################################
# Google Chrome and Fonts (needed for Karma and Puppeteer tests, respectively). #
#################################################################################

# TODO(borenet): we should be able to use this from rules_browsers.
load("//bazel/external:google_chrome.bzl", "google_chrome")

google_chrome(name = "google_chrome")

################################################
# bazel-toolchains rbe_configs_gen (prebuilt). #
################################################

http_file(
    name = "rbe_configs_gen_linux_amd64",
    downloaded_file_path = "rbe_configs_gen",
    executable = True,
    sha256 = "1206e8a79b41cb22524f73afa4f4ee648478f46ef6990d78e7cc953665a1db89",
    urls = gcs_mirror_url(
        ext = "",
        sha256 = "1206e8a79b41cb22524f73afa4f4ee648478f46ef6990d78e7cc953665a1db89",
        url = "https://github.com/bazelbuild/bazel-toolchains/releases/download/v5.1.2/rbe_configs_gen_linux_amd64",
    ),
)

###########
# protoc. #
###########

# The following archives were taken from
# https://github.com/protocolbuffers/protobuf/releases/tag/v21.12.
PROTOC_BUILD_FILE_CONTENT = """
exports_files(["bin/protoc"], visibility = ["//visibility:public"])
"""

http_archive(
    name = "protoc_linux_x64",
    build_file_content = PROTOC_BUILD_FILE_CONTENT,
    sha256 = "3a4c1e5f2516c639d3079b1586e703fc7bcfa2136d58bda24d1d54f949c315e8",
    urls = gcs_mirror_url(
        sha256 = "3a4c1e5f2516c639d3079b1586e703fc7bcfa2136d58bda24d1d54f949c315e8",
        url = "https://github.com/protocolbuffers/protobuf/releases/download/v21.12/protoc-21.12-linux-x86_64.zip",
    ),
)

http_archive(
    name = "protoc_mac_x64",
    build_file_content = PROTOC_BUILD_FILE_CONTENT,
    sha256 = "9448ff40278504a7ae5139bb70c962acc78c32d8fc54b4890a55c14c68b9d10a",
    urls = gcs_mirror_url(
        sha256 = "9448ff40278504a7ae5139bb70c962acc78c32d8fc54b4890a55c14c68b9d10a",
        url = "https://github.com/protocolbuffers/protobuf/releases/download/v21.12/protoc-21.12-osx-x86_64.zip",
    ),
)
