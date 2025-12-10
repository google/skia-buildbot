workspace(
    name = "skia_infra",
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

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

# #################################################################################
# # Google Chrome and Fonts (needed for Karma and Puppeteer tests, respectively). #
# #################################################################################

# TODO(borenet): we should be able to use this from rules_browsers.
load("//bazel/external:google_chrome.bzl", "google_chrome")

google_chrome(name = "google_chrome")
