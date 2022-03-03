workspace(
    name = "skia_infra",

    # Must be kept in sync with the npm_install rules defined below invoked below.
    managed_directories = {
        "@npm": ["node_modules"],
    },
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Read the instructions in //bazel/rbe/README.md before updating this repository.
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
    url = "https://github.com/bazelbuild/buildtools/archive/4.0.0.zip",
)

##############################
# Go rules and dependencies. #
##############################

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2b1641428dff9018f9e85c0384f03ec6c10660d935b750e3fa1492a281a53b0f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "de69a09dc70417580aabf20a28619bb3ef60d038470c7cf8442fafcf627c21cb",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.24.0/bazel-gazelle-v0.24.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.24.0/bazel-gazelle-v0.24.0.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("//:go_repositories.bzl", "go_repositories")

# gazelle:repository_macro go_repositories.bzl%go_repositories
go_repositories()

go_rules_dependencies()

# Gazelle fails for toolchain versions < 1.14 with an error like the following:
#
#     gazelle: [...]: go: updates to go.mod needed, disabled by -mod=readonly
go_register_toolchains(version = "1.17.2")

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
    urls = ["https://github.com/googleapis/googleapis/archive/143084a2624b6591ee1f9d23e7f5241856642f4d.zip"],
)

# Needed by @com_github_bazelbuild_remote_apis for gRPC.
http_archive(
    name = "com_github_grpc_grpc",
    sha256 = "b391a327429279f6f29b9ae7e5317cd80d5e9d49cc100e6d682221af73d984a6",
    strip_prefix = "grpc-93e8830070e9afcbaa992c75817009ee3f4b63a0",  # v1.24.3 with fixes
    urls = ["https://github.com/grpc/grpc/archive/93e8830070e9afcbaa992c75817009ee3f4b63a0.zip"],
)

load("@com_github_grpc_grpc//bazel:grpc_deps.bzl", "grpc_deps")

grpc_deps()

###################################################
# JavaScript / TypeScript rules and dependencies. #
###################################################

http_archive(
    name = "build_bazel_rules_nodejs",
    sha256 = "965ee2492a2b087cf9e0f2ca472aeaf1be2eb650e0cfbddf514b9a7d3ea4b02a",
    urls = ["https://github.com/bazelbuild/rules_nodejs/releases/download/5.2.0/rules_nodejs-5.2.0.tar.gz"],
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
    url = "https://github.com/bazelbuild/rules_sass/archive/1.49.8.zip",
)

load("@io_bazel_rules_sass//:defs.bzl", "sass_repositories")

sass_repositories()

##################################
# Docker rules and dependencies. #
##################################

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "59d5b42ac315e7eadffa944e86e90c2990110a1c8075f1cd145f487e999d22b3",
    strip_prefix = "rules_docker-0.17.0",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.17.0/rules_docker-v0.17.0.tar.gz"],
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

# Pulls the gcr.io/google/rbe-ubuntu16-04 container, used as the base container for our custom RBE
# toolchain container.
container_pull(
    name = "google_debian10",
    digest = "sha256:96a0145e8bb84d6886abfb9f6a955d9ab3f8b1876b8f7572273598c86e902983",
    registry = "gcr.io",
    repository = "cloud-marketplace/google/debian10",
)

# Pulls the gcr.io/skia-public/skia-wasm-release container with the Skia WASM build.
container_pull(
    name = "container_pull_skia_wasm",
    registry = "gcr.io",
    repository = "skia-public/skia-wasm-release",
    # The container_pull documentation[1] recommends specifying a digest (via the "digest" argument)
    # for reproducible builds. Specifying "head" ends up not working well because of Bazel caching.
    # We should only need to update this if CanvasKit adds new APIs that are depended on by
    # our webapps, and that is not too often.
    tag = "7b5c0de7ceb1b5238bad735037e48b3726633dc0",
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

# Pulls the node:17-alpine container, needed by jsdoc.
container_pull(
    name = "kubectl",
    digest = "sha256:fb1a8540f657d76f980c75e59fa95fa3683f9f7eadeea6fbdff099968bfcadca",
    registry = "gcr.io",
    repository = "cloud-builders/kubectl",
)

##############################
# Packages for RBE container #
##############################
# The following http_archives are used to download and verify files that will be installed on
# our RBE container.
http_archive(
    name = "go_sdk_external",
    # We are downloading a tar file of pre-compiled executables, libraries and such that does
    # NOT have a BUILD file for Bazel to read. As such, we can specify one here using
    # build_file_content that makes all the contents of the the Golang SDK tar file available
    # as a target called @go_sdk_external//:extracted_files
    #
    # Debugging tip: make an intentional typo in build_file_content and then try to `bazel build`
    # something that depends on this. The error message will show where the archive is being
    # downloaded/extracted in your bazel cache, which can help manual inspection.
    build_file_content = """
filegroup(
    name = "extracted_files",
    srcs = glob(["go/*"], exclude_directories=0),
    visibility = ["//visibility:public"]
)""",
    # From https://golang.org/dl/
    sha256 = "dab7d9c34361dc21ec237d584590d72500652e7c909bf082758fb63064fca0ef",
    urls = ["https://golang.org/dl/go1.17.1.linux-amd64.tar.gz"],
)

http_archive(
    name = "cockroachdb_external",
    # We are downloading a zip file of pre-compiled executables, libraries and such that does
    # NOT have a BUILD file for Bazel to read. As such, we can specify one here using
    # build_file_content that makes all the contents of the the Android NDK zip file available
    # as a target called @android_ndk_external//:extracted_files
    build_file_content = """
filegroup(
    name = "extracted_exe",
    srcs = ["cockroach-v21.1.9.linux-amd64/cockroach"],
    visibility = ["//visibility:public"]
)""",
    # https://www.cockroachlabs.com/docs/v21.1/install-cockroachdb-linux does not currently
    # provide SHA256 signatures. kjlubick@ downloaded this file and computed this sha256 signature.
    sha256 = "05293e76dfb6443790117b6c6c05b1152038b49c83bd4345589e15ced8717be3",
    url = "https://binaries.cockroachdb.com/cockroach-v21.1.9.linux-amd64.tgz",
)
