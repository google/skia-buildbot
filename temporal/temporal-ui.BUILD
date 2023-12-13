# BUILD file for Temporal UI server sources in the github
# https://github.com/temporalio/ui-server

load("@io_bazel_rules_docker//container:container.bzl", "container_image")
load("@io_bazel_rules_docker//docker/util:run.bzl", "container_run_and_extract")
load("@rules_pkg//:pkg.bzl", "pkg_tar")

pkg_tar(
    name = "sources",
    srcs = glob(["**/*"]),
    mode = "0755",
    package_dir = "/temporalui-srcs",
    strip_prefix = ".",  # Preserve the folder structure.
)

container_image(
    name = "temporalui-srcs",
    base = "@golang//image",
    tars = [
        ":sources",
    ],
    workdir = "/temporalui-srcs",
)

container_run_and_extract(
    name = "ui-server",
    commands = [
        "export CGO_ENABLED=0 GOOS=linux",
        "go build -o ui-server ./cmd/server",
    ],
    extract_file = "/temporalui-srcs/ui-server",
    image = ":temporalui-srcs.tar",
)

# The built temporal UI server
pkg_tar(
    name = "temporal-ui-server",
    srcs = [
        ":ui-server/temporalui-srcs/ui-server",
    ],
    package_dir = "/etc/temporal",
    visibility = ["//visibility:public"],
)
