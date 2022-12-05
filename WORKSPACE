workspace(
    name = "skia_infra",

    # Must be kept in sync with the npm_install rules invoked below.
    managed_directories = {
        "@npm": ["node_modules"],
    },
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

###############
# Buildifier. #
###############

http_archive(
    name = "com_github_bazelbuild_buildtools",
    sha256 = "2adaafee16c53b80adff742b88bc90b2a5e99bf6889a5d82f22ef66655dc467b",
    strip_prefix = "buildtools-4.0.0",
    urls = gcs_mirror_url(
        sha256 = "2adaafee16c53b80adff742b88bc90b2a5e99bf6889a5d82f22ef66655dc467b",
        url = "https://github.com/bazelbuild/buildtools/archive/4.0.0.zip",
    ),
)

#################
# Python rules. #
#################

http_archive(
    name = "rules_python",
    sha256 = "cdf6b84084aad8f10bf20b46b77cb48d83c319ebe6458a18e9d2cebf57807cdd",
    strip_prefix = "rules_python-0.8.1",
    urls = gcs_mirror_url(
        sha256 = "cdf6b84084aad8f10bf20b46b77cb48d83c319ebe6458a18e9d2cebf57807cdd",
        url = "https://github.com/bazelbuild/rules_python/archive/refs/tags/0.8.1.tar.gz",
    ),
)

load("@rules_python//python:repositories.bzl", "python_register_toolchains")

# Hermetically downloads Python 3.
python_register_toolchains(
    name = "python3_10",
    # Taken from
    # https://github.com/bazelbuild/rules_python/blob/63805ab7a65b90c4723ecbe18f2c88da714e5d7a/python/versions.bzl#L94.
    python_version = "3.10",
)

##############################
# Go rules and dependencies. #
##############################

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "d6b2513456fe2229811da7eb67a444be7785f5323c6708b38d851d2b51e54d83",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.30.0/rules_go-v0.30.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.30.0/rules_go-v0.30.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "5982e5463f171da99e3bdaeff8c0f48283a7a5f396ec5282910b9e8a49c0dd7e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.25.0/bazel-gazelle-v0.25.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.25.0/bazel-gazelle-v0.25.0.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("//:go_repositories.bzl", "go_repositories")

# gazelle:repository_macro go_repositories.bzl%go_repositories
go_repositories()

go_rules_dependencies()

go_register_toolchains(version = "1.18")

gazelle_dependencies()

##########################
# Other Go dependencies. #
##########################

# Needed by @com_github_bazelbuild_remote_apis.
load("@com_github_bazelbuild_remote_apis//:repository_rules.bzl", "switched_rules_by_language")

switched_rules_by_language(
    name = "bazel_remote_apis_imports",
    go = True,
)

# Needed by @com_github_bazelbuild_remote_apis.
http_archive(
    name = "com_google_protobuf",
    sha256 = "d0f5f605d0d656007ce6c8b5a82df3037e1d8fe8b121ed42e536f569dec16113",
    strip_prefix = "protobuf-3.14.0",
    urls = [
        "https://mirror.bazel.build/github.com/protocolbuffers/protobuf/archive/v3.14.0.tar.gz",
        "https://github.com/protocolbuffers/protobuf/archive/v3.14.0.tar.gz",
    ],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

# Needed by @com_github_bazelbuild_remote_apis for the googleapis protos.
http_archive(
    name = "googleapis",
    build_file = "BUILD.googleapis",
    sha256 = "7b6ea252f0b8fb5cd722f45feb83e115b689909bbb6a393a873b6cbad4ceae1d",
    strip_prefix = "googleapis-143084a2624b6591ee1f9d23e7f5241856642f4d",
    urls = gcs_mirror_url(
        sha256 = "7b6ea252f0b8fb5cd722f45feb83e115b689909bbb6a393a873b6cbad4ceae1d",
        url = "https://github.com/googleapis/googleapis/archive/143084a2624b6591ee1f9d23e7f5241856642f4d.zip",
    ),
)

# Needed by @com_github_bazelbuild_remote_apis for gRPC.
http_archive(
    name = "com_github_grpc_grpc",
    sha256 = "b391a327429279f6f29b9ae7e5317cd80d5e9d49cc100e6d682221af73d984a6",
    strip_prefix = "grpc-93e8830070e9afcbaa992c75817009ee3f4b63a0",  # v1.24.3 with fixes
    urls = gcs_mirror_url(
        sha256 = "b391a327429279f6f29b9ae7e5317cd80d5e9d49cc100e6d682221af73d984a6",
        url = "https://github.com/grpc/grpc/archive/93e8830070e9afcbaa992c75817009ee3f4b63a0.zip",
    ),
)

load("@com_github_grpc_grpc//bazel:grpc_deps.bzl", "grpc_deps")

grpc_deps()

###################################################
# JavaScript / TypeScript rules and dependencies. #
###################################################

http_archive(
    name = "build_bazel_rules_nodejs",
    sha256 = "0fad45a9bda7dc1990c47b002fd64f55041ea751fafc00cd34efb96107675778",
    urls = gcs_mirror_url(
        sha256 = "0fad45a9bda7dc1990c47b002fd64f55041ea751fafc00cd34efb96107675778",
        url = "https://github.com/bazelbuild/rules_nodejs/releases/download/5.5.0/rules_nodejs-5.5.0.tar.gz",
    ),
)

load("@build_bazel_rules_nodejs//:repositories.bzl", "build_bazel_rules_nodejs_dependencies")

build_bazel_rules_nodejs_dependencies()

load("@build_bazel_rules_nodejs//:index.bzl", "node_repositories", "npm_install")

node_repositories(
    node_version = "16.12.0",
    # We don't use Yarn directly, but the Bazel rules in the rules_nodejs repository do.
    yarn_version = "1.22.11",
)

# The npm_install rule manages the node_modules directory, and runs anytime the package.json or
# package-lock.json file changes. It also extracts any Bazel rules distributed in an NPM package.
#
# There must be one npm_install rule for each package.json file in this repository. Any node_modules
# directories managed by npm_install rules must be mentioned in the workspace() rule at the top of
# this file.
npm_install(
    name = "npm",
    exports_directories_only = False,
    package_json = "//:package.json",
    package_lock_json = "//:package-lock.json",
    symlink_node_modules = True,
)

load(
    "@build_bazel_rules_nodejs//toolchains/esbuild:esbuild_repositories.bzl",
    "esbuild_repositories",
)

esbuild_repositories(npm_repository = "npm")

################################
# Sass rules and dependencies. #
################################

http_archive(
    name = "io_bazel_rules_sass",
    sha256 = "6cca1c3b77185ad0a421888b90679e345d7b6db7a8c9c905807fe4581ea6839a",
    strip_prefix = "rules_sass-1.49.8",
    urls = gcs_mirror_url(
        sha256 = "6cca1c3b77185ad0a421888b90679e345d7b6db7a8c9c905807fe4581ea6839a",
        url = "https://github.com/bazelbuild/rules_sass/archive/1.49.8.zip",
    ),
)

load("@io_bazel_rules_sass//:defs.bzl", "sass_repositories")

sass_repositories()

##################################
# Docker rules and dependencies. #
##################################

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820",
    strip_prefix = "rules_docker-0.24.0",
    urls = gcs_mirror_url(
        sha256 = "27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820",
        url = "https://github.com/bazelbuild/rules_docker/releases/download/v0.24.0/rules_docker-v0.24.0.tar.gz",
    ),
)

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

# This is required by the toolchain_container rule.
load(
    "@io_bazel_rules_docker//repositories:go_repositories.bzl",
    container_go_deps = "go_deps",
)

container_go_deps()

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
)

# Provides the pkg_tar rule, needed by the skia_app_container macro.
#
# See https://github.com/bazelbuild/rules_pkg/tree/main/pkg.
http_archive(
    name = "rules_pkg",
    sha256 = "038f1caa773a7e35b3663865ffb003169c6a71dc995e39bf4815792f385d837d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz",
        "https://github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz",
    ],
)

load("@rules_pkg//:deps.bzl", "rules_pkg_dependencies")

rules_pkg_dependencies()

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

######################
# Docker containers. #
######################

# Pulls the gcr.io/skia-public/skia-wasm-release container with the Skia WASM build.
container_pull(
    name = "container_pull_skia_wasm",
    registry = "gcr.io",
    repository = "skia-public/skia-wasm-release",
    # The container_pull documentation[1] recommends specifying a digest (via the "digest" argument)
    # for reproducible builds. Specifying "head" ends up not working well because of Bazel caching.
    # We should only need to update this if CanvasKit adds new APIs that are depended on by
    # our webapps, and that is not too often.
    tag = "2508d582b5b68029f03b61b7103b3140f95bd071",
)

# This is an arbitrary version of the public Alpine image. Given our current rules, we must pull
# a docker container and extract some files, even if we are just building local versions (e.g.
# of debugger or skottie), so this is the image for that.
container_pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
    registry = "index.docker.io",
    repository = "alpine",
)

# Pulls the gcr.io/skia-public/basealpine container, needed by the skia_app_container macro.
container_pull(
    name = "basealpine",
    digest = "sha256:35a26930eb37b90cb0bdf69050e363bd749b56656963b78c8c4b4758a5aea8fa",
    registry = "gcr.io",
    repository = "skia-public/basealpine",
)

# Pulls the gcr.io/skia-public/base-cipd container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "base-cipd",
    digest = "sha256:0ae30b768fb1bdcbea5b6721075b758806c4076a74a8a99a67ff3632df87cf5a",
    registry = "gcr.io",
    repository = "skia-public/base-cipd",
)

# Pulls the gcr.io/skia-public/cd-base container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "cd-base",
    digest = "sha256:17e18164238a4162ce2c30b7328a7e44fbe569e56cab212ada424dc7378c1f5f",
    registry = "gcr.io",
    repository = "skia-public/cd-base",
)

# Pulls the gcr.io/skia-public/skia-build-tools container, needed by some apps that
# build skia.
container_pull(
    name = "skia-build-tools",
    digest = "sha256:28cc48a073ac1f35f468c1b725e331b626791b35edb18696f30891c4f047d236",
    registry = "gcr.io",
    repository = "skia-public/skia-build-tools",
)

# Pulls the gcr.io/skia-public/docsyserver-base container, needed by docsyserver.
container_pull(
    name = "docsyserver-base",
    digest = "sha256:ca63ba5a92e1adbe49eb6e6e1262ee4724e572f87e54eea01737cbb2a73fde6c",
    registry = "gcr.io",
    repository = "skia-public/docsyserver-base",
)

# Pulls the envoyproxy/envoy-alpine:v1.16.1 container, needed by skfe.
container_pull(
    name = "envoy_alpine",
    digest = "sha256:061559f887b6b7980ea1ebb5af636079858d8b0f51cd803b9fe16f87811ff7d3",
    registry = "index.docker.io",
    repository = "envoyproxy/envoy-alpine",
)

# Pulls the node:17-alpine container, needed by jsdoc.
container_pull(
    name = "node_alpine",
    digest = "sha256:44b4db12ba2899f92786aa7e98782eb6430e81d92488c59144a567853185c2bb",
    registry = "index.docker.io",
    repository = "node",
)

# Pulls the cloud-builders/kubectl container, needed by apps that use kubectl.
container_pull(
    name = "kubectl",
    digest = "sha256:66fb5ffddfb7d9dc02daf3cdc809d548ea7cbab53bf67ff25f748d8559323796",
    registry = "gcr.io",
    repository = "cloud-builders/kubectl",
)

# Pulls the gcr.io/google.com/cloudsdktool/cloud-sdk:latest container needed by Perf backup.
container_pull(
    name = "cloudsdk",
    digest = "sha256:900b74f1fb2c9f93c6d4b121a7f23981143496f36aacb72e596ccaedad640cf1",  # @latest as of Apr 27, 2022.
    registry = "gcr.io",
    repository = "google.com/cloudsdktool/cloud-sdk",
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
    # From https://chrome-infra-packages.appspot.com/p/infra/tools/luci/vpython/linux-amd64/+/git_revision:7989c7a87b25083bd8872f9216ba4819c18ab097
    sha256 = "1de06f1727bde7ef9eaae901944adead46dd2b7ddda1e962fff29ee431b0e746",
    tag = "git_revision:7989c7a87b25083bd8872f9216ba4819c18ab097",
)

cipd_install(
    name = "cpython3_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/cpython3/linux-amd64",
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/cpython3/linux-amd64/+/version:2@3.8.10.chromium.19
    sha256 = "4ba68650a271a80a565a619ed2419f4cf1344525b63798608ce3b8cef63a9244",
    tag = "version:2@3.8.10.chromium.19",
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

#################################################################################
# Google Chrome and Fonts (needed for Karma and Puppeteer tests, respectively). #
#################################################################################

load("//bazel/external:google_chrome.bzl", "google_chrome")

google_chrome(name = "google_chrome")

##########################
# Buildifier (prebuilt). #
##########################

http_file(
    name = "buildifier_linux_amd64",
    downloaded_file_path = "buildifier",
    executable = True,
    sha256 = "52bf6b102cb4f88464e197caac06d69793fa2b05f5ad50a7e7bf6fbd656648a3",
    urls = gcs_mirror_url(
        ext = "",
        sha256 = "52bf6b102cb4f88464e197caac06d69793fa2b05f5ad50a7e7bf6fbd656648a3",
        url = "https://github.com/bazelbuild/buildtools/releases/download/5.1.0/buildifier-linux-amd64",
    ),
)

http_file(
    name = "buildifier_macos_arm64",
    downloaded_file_path = "buildifier",
    executable = True,
    sha256 = "745feb5ea96cb6ff39a76b2821c57591fd70b528325562486d47b5d08900e2e4",
    urls = gcs_mirror_url(
        ext = "",
        sha256 = "745feb5ea96cb6ff39a76b2821c57591fd70b528325562486d47b5d08900e2e4",
        url = "https://github.com/bazelbuild/buildtools/releases/download/5.1.0/buildifier-darwin-arm64",
    ),
)

http_file(
    name = "buildifier_macos_amd64",
    downloaded_file_path = "buildifier",
    executable = True,
    sha256 = "c9378d9f4293fc38ec54a08fbc74e7a9d28914dae6891334401e59f38f6e65dc",
    urls = gcs_mirror_url(
        ext = "",
        sha256 = "c9378d9f4293fc38ec54a08fbc74e7a9d28914dae6891334401e59f38f6e65dc",
        url = "https://github.com/bazelbuild/buildtools/releases/download/5.1.0/buildifier-darwin-amd64",
    ),
)

###########
# protoc. #
###########

# The following archives were taken from
# https://github.com/protocolbuffers/protobuf/releases/tag/v3.3.0. In order to prevent diffs, the
# version should match that of the protoc CIPD package, see
# https://skia.googlesource.com/skia/+/e7cdb8e4e38f9b6af38ad65c6770ada3d42656d7/infra/bots/assets/protoc/create.py#16.
#
# Note that protoc v3.3.0 precedes M1 Macs and thus there is no arm64 binary. We can fix this by
# updating protoc to a more recent version.

PROTOC_BUILD_FILE_CONTENT = """
exports_files(["bin/protoc"], visibility = ["//visibility:public"])
"""

http_archive(
    name = "protoc_linux_x64",
    build_file_content = PROTOC_BUILD_FILE_CONTENT,
    sha256 = "feb112bbc11ea4e2f7ef89a359b5e1c04428ba6cfa5ee628c410eccbfe0b64c3",
    urls = gcs_mirror_url(
        sha256 = "feb112bbc11ea4e2f7ef89a359b5e1c04428ba6cfa5ee628c410eccbfe0b64c3",
        url = "https://github.com/protocolbuffers/protobuf/releases/download/v3.3.0/protoc-3.3.0-linux-x86_64.zip",
    ),
)

http_archive(
    name = "protoc_mac_x64",
    build_file_content = PROTOC_BUILD_FILE_CONTENT,
    sha256 = "d752ba0ea67239e327a48b2f23da0e673928a9ff06ee530319fc62200c0aff89",
    urls = gcs_mirror_url(
        sha256 = "d752ba0ea67239e327a48b2f23da0e673928a9ff06ee530319fc62200c0aff89",
        url = "https://github.com/protocolbuffers/protobuf/releases/download/v3.3.0/protoc-3.3.0-osx-x86_64.zip",
    ),
)
