# BUILD file for Temporal CLI sources in the github
# https://github.com/temporalio/cli

load("@io_bazel_rules_docker//container:container.bzl", "container_image")
load("@io_bazel_rules_docker//docker/util:run.bzl", "container_run_and_extract")
load("@rules_pkg//:pkg.bzl", "pkg_tar")

pkg_tar(
    name = "sources",
    srcs = glob(["**/*"]),
    mode = "0755",
    package_dir = "/temporal-srcs",
    strip_prefix = ".",  # Preserve the folder structure.
)

container_image(
    name = "temporal-srcs",
    base = "@golang//image",
    tars = [
        ":sources",
    ],
    workdir = "/temporal-srcs",
)

container_run_and_extract(
    name = "temporal-bins",
    commands = [
        "CGO_ENABLED=0 go build -o /temporal -ldflags '-s -w' ./cmd/temporal",
    ],
    extract_file = "/temporal",
    image = ":temporal-srcs.tar",
)

# The built temporal CLI tool
pkg_tar(
    name = "temporal-cli",
    srcs = [
        ":temporal-bins/temporal",
    ],
    package_dir = "/etc/temporal",
    visibility = ["//visibility:public"],
)
