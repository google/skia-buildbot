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

#################
# Python rules. #
#################

http_archive(
    name = "rules_python",
    sha256 = "c68bdc4fbec25de5b5493b8819cfc877c4ea299c0dcb15c244c5a00208cde311",
    strip_prefix = "rules_python-0.31.0",
    urls = gcs_mirror_url(
        sha256 = "c68bdc4fbec25de5b5493b8819cfc877c4ea299c0dcb15c244c5a00208cde311",
        # Update after a release with https://github.com/bazelbuild/rules_python/pull/1032 lands
        url = "https://github.com/bazelbuild/rules_python/archive/refs/tags/0.31.0.tar.gz",
    ),
)

load("@rules_python//python:repositories.bzl", "py_repositories", "python_register_toolchains")

# Load transitive dependencies for rules_python.
py_repositories()

# Hermetically downloads Python 3.
python_register_toolchains(
    name = "python3_11",
    # Our Louhi builds run as root in order to prevent "permission denied"
    # errors when attempting to write to mounted directories controlled by
    # Google Cloud Build.
    ignore_root_user_error = True,
    # Taken from
    # https://github.com/bazelbuild/rules_python/blob/1f17637b88489a5c35a5c83595c0e8dbb6d983e9/python/versions.bzl#L372.
    python_version = "3.11",
)

load("@rules_python//python:pip.bzl", "pip_parse")

pip_parse(
    name = "pypi",
    requirements_lock = "//:requirements.txt",
)

load("@pypi//:requirements.bzl", "install_deps")

install_deps()

##############################
# Go rules and dependencies. #
##############################

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "f2d15bea3e241aa0e3a90fb17a82e6a8ab12214789f6aeddd53b8d04316d2b7c",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.54.0/rules_go-v0.54.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.54.0/rules_go-v0.54.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "8ad77552825b078a10ad960bec6ef77d2ff8ec70faef2fd038db713f410f5d87",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.38.0/bazel-gazelle-v0.38.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.38.0/bazel-gazelle-v0.38.0.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("//:go_repositories.bzl", "go_repositories")

# gazelle:repository_macro go_repositories.bzl%go_repositories
go_repositories()

go_rules_dependencies()

go_register_toolchains(version = "1.24.2")

gazelle_dependencies()

##########################
# Other Go dependencies. #
##########################

load("//bazel/external:go_googleapis_compatibility_hack.bzl", "go_googleapis_compatibility_hack")

# Compatibility hack to make the github.com/bazelbuild/remote-apis Go module work with rules_go
# v0.41.0 or newer. See the go_googleapis() rule's docstring for details.
go_googleapis_compatibility_hack(
    name = "go_googleapis",
)

# Needed by @com_github_bazelbuild_remote_apis.
http_archive(
    name = "com_google_protobuf",
    sha256 = "da288bf1daa6c04d03a9051781caa52aceb9163586bff9aa6cfb12f69b9395aa",
    strip_prefix = "protobuf-27.0",
    urls = gcs_mirror_url(
        sha256 = "da288bf1daa6c04d03a9051781caa52aceb9163586bff9aa6cfb12f69b9395aa",
        url = "https://github.com/protocolbuffers/protobuf/releases/download/v27.0/protobuf-27.0.tar.gz",
    ),
)

# Originally, we pulled protobuf dependencies as follows:
#
#     load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")
#     protobuf_deps()
#
# The protobuf_deps() macro brings in a bunch of dependencies, but by copying the macro body here
# and removing dependencies one by one, "rules_proto" was identified as the only dependency that is
# required to build this repository.

http_archive(
    name = "com_google_absl",
    sha256 = "f49929d22751bf70dd61922fb1fd05eb7aec5e7a7f870beece79a6e28f0a06c1",
    strip_prefix = "abseil-cpp-4a2c63365eff8823a5221db86ef490e828306f9d",
    # Abseil LTS 20240116.0
    urls = ["https://github.com/abseil/abseil-cpp/archive/4a2c63365eff8823a5221db86ef490e828306f9d.zip"],
)

http_archive(
    name = "rules_proto",
    sha256 = "6fb6767d1bef535310547e03247f7518b03487740c11b6c6adb7952033fe1295",
    strip_prefix = "rules_proto-6.0.2",
    url = "https://github.com/bazelbuild/rules_proto/releases/download/6.0.2/rules_proto-6.0.2.tar.gz",
)

load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies")

rules_proto_dependencies()

load("@rules_proto//proto:setup.bzl", "rules_proto_setup")

rules_proto_setup()

load("@rules_proto//proto:toolchains.bzl", "rules_proto_toolchains")

rules_proto_toolchains()

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

# Needed by @com_github_bazelbuild_remote_apis for the googleapis protos.
http_archive(
    name = "googleapis",
    build_file = "//bazel/external:googleapis.BUILD",
    sha256 = "b28c13e99001664eac5f1fb81b44d912d19fbc041e30772263251da131f6573c",
    strip_prefix = "googleapis-bb964feba5980ed70c9fb8f84fe6e86694df65b0",
    urls = gcs_mirror_url(
        sha256 = "b28c13e99001664eac5f1fb81b44d912d19fbc041e30772263251da131f6573c",
        # b/267219467
        url = "https://github.com/googleapis/googleapis/archive/bb964feba5980ed70c9fb8f84fe6e86694df65b0.zip",
    ),
)

load("@googleapis//:repository_rules.bzl", googleapis_imports_switched_rules_by_language = "switched_rules_by_language")

googleapis_imports_switched_rules_by_language(
    name = "com_google_googleapis_imports",
    go = True,
    grpc = True,
)

# Needed by @com_github_bazelbuild_remote_apis for gRPC.
http_archive(
    name = "com_github_grpc_grpc",
    sha256 = "b391a327429279f6f29b9ae7e5317cd80d5e9d49cc100e6d682221af73d984a6",
    strip_prefix = "grpc-93e8830070e9afcbaa992c75817009ee3f4b63a0",  # v1.24.3 with fixes
    urls = gcs_mirror_url(
        sha256 = "b391a327429279f6f29b9ae7e5317cd80d5e9d49cc100e6d682221af73d984a6",
        # Fix after https://github.com/grpc/grpc/issues/32259 is resolved
        url = "https://github.com/grpc/grpc/archive/93e8830070e9afcbaa992c75817009ee3f4b63a0.zip",
    ),
)

# Originally, we pulled gRPC dependencies as follows:
#
#     load("@com_github_grpc_grpc//bazel:grpc_deps.bzl", "grpc_deps")
#     grpc_deps()
#
# The grpc_deps() macro brings in a bunch of dependencies, but by copying the macro body here
# and removing dependencies one by one, "zlib" was identified as the only dependency that is
# required to build this repository.
http_archive(
    name = "zlib",
    build_file = "@com_github_grpc_grpc//third_party:zlib.BUILD",
    strip_prefix = "zlib-cacf7f1d4e3d44d871b605da3b647f07d718623f",
    urls = gcs_mirror_url(
        sha256 = "6d4d6640ca3121620995ee255945161821218752b551a1a180f4215f7d124d45",
        url = "https://github.com/madler/zlib/archive/cacf7f1d4e3d44d871b605da3b647f07d718623f.tar.gz",
    ),
)

http_archive(
    name = "com_github_temporal",
    build_file = "//temporal:temporal.BUILD",
    strip_prefix = "./temporal-1.23.1",
    urls = gcs_mirror_url(
        sha256 = "3110fa0df19de58d6afa9b1af3dd7274a5e37d5082e424c114d7b29c696ceae1",
        url = "https://github.com/temporalio/temporal/archive/refs/tags/v1.23.1.tar.gz",
    ),
)

http_archive(
    name = "com_github_temporal_cli",
    build_file = "//temporal:temporal-cli.BUILD",
    strip_prefix = "./cli-0.13.1",
    urls = gcs_mirror_url(
        sha256 = "9d8812c96d3404490659fec3915dcd23c4142b421ef4cb7e9622bd9a459e1f74",
        url = "https://github.com/temporalio/cli/archive/refs/tags/v0.13.1.tar.gz",
    ),
)

http_archive(
    name = "com_github_temporal_ui",
    build_file = "//temporal:temporal-ui.BUILD",
    strip_prefix = "./ui-server-2.27.3",
    urls = gcs_mirror_url(
        sha256 = "b9ecf1afadce3e693c852b4bbe0dce5639998c10384692ca23b6a94e0d64642d",
        url = "https://github.com/temporalio/ui-server/archive/refs/tags/v2.27.3.tar.gz",
    ),
)

##################################
# Docker rules and dependencies. #
##################################

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "b1e80761a8a8243d03ebca8845e9cc1ba6c82ce7c5179ce2b295cd36f7e394bf",
    urls = gcs_mirror_url(
        sha256 = "b1e80761a8a8243d03ebca8845e9cc1ba6c82ce7c5179ce2b295cd36f7e394bf",
        url = "https://github.com/bazelbuild/rules_docker/releases/download/v0.25.0/rules_docker-v0.25.0.tar.gz",
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

# This is a pinned version of JS fiddle - we use the canvaskit.js/canvaskit.wasm inside it
# when running apps (e.g. skottie.skia.org) locally. All our apps (except debugger) use the stock
# version of CanvasKit, so they can share this. If there is an update to CanvasKit APIs and we want
# to test them out locally, we should update this to a newer version. See the k8s-config repo
# for a recent commit to use. You can also get the latest sha256 by either:
# 1. Running `docker pull gcr.io/skia-public/jsfiddle-final:latest` and looking for the sha256
# in the log. This requires docker to be installed and authenticated through gcloud.
# 2. Going to https://skia.googlesource.com/k8s-config/+/refs/heads/main/skia-infra-public/jsfiddle.yaml
# and looking for sha256 on the jsfiddle-final image.
container_pull(
    name = "pinned_jsfiddle",
    digest = "sha256:843c9396daa61f4674b1868f3a27d6490d624fc0038e5912efec77ba94df6cf3",
    registry = "gcr.io",
    repository = "skia-public/jsfiddle-final",
)

# Debugger's version of CanvasKit is built with different flags
container_pull(
    name = "pinned_debugger",
    digest = "sha256:6e0291dcb56a7da29ebcfc332384fc8ea5c4beb84aee985da31d317cef41bb5e",
    registry = "gcr.io",
    repository = "skia-public/debugger-app-final",
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
    digest = "sha256:5474db9668fb91cbf71b160e41047ca48a7686a25b7e3d38da9f1c720683e2c7",
    registry = "gcr.io",
    repository = "skia-public/basealpine",
)

# Pulls the gcr.io/skia-public/base-cipd container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "base-cipd",
    digest = "sha256:d775388db67ebb4d9cd36a759aaee785a3cf4a3999120f57ed9135e1445eb9d5",
    registry = "gcr.io",
    repository = "skia-public/base-cipd",
)

# Pulls the gcr.io/skia-public/cd-base container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "cd-base",
    digest = "sha256:59412eeaf3336f14a8fef1b0b02134c9877bf5aba16549e8c59d5e4de29719b8",
    registry = "gcr.io",
    repository = "skia-public/cd-base",
)

# Pulls the gcr.io/skia-public/docsyserver-base container, needed by docsyserver.
container_pull(
    name = "docsyserver-base",
    digest = "sha256:c92739a43735be44a28135c9eac75753c64c2d3b9ba5c97efcaf57d5bd2ab12a",
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

# Pulls the node alpine container, needed by npm-audit-mirror.
container_pull(
    name = "node_alpine",
    digest = "sha256:1e168eaa83acc4b002f7b91bd3584f6800c84737337fe665224c963ed6b5b1c0",  # index.docker.io/node:current-alpine3.21 as of Oct 2, 2025.
    registry = "index.docker.io",
    repository = "node",
)

# Pulls the https://gcr.io/cloud-builders/kubectl container, needed by apps that use kubectl.
container_pull(
    name = "kubectl",
    digest = "sha256:63553d791cbdd3aa9fc2bc0b3a6a6d33130c1b8927b2db368c756aa45c89a356",  # 25 Oct 2023
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

container_pull(
    name = "golang",
    digest = "sha256:80ccdc8f8ac8d819cdbc15a33334125e0288c09ac030307dcd893d2b5c6179ae",
    import_tags = ["1.21.3"],
    registry = "google-go.pkg.dev",
    repository = "golang",
)

container_pull(
    name = "fiddler-build-skia",
    digest = "sha256:8eea043f967e6850c3ea97d1b1a6e9a5faa704fdc86138ac5444f56af0b258c5",
    registry = "gcr.io",
    repository = "skia-public/fiddler-build-skia",
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
