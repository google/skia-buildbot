# BUILD file for Temporal sources in the github
# https://github.com/temporalio/temporal

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
        "export CGO_ENABLED=0 GOOS=linux",
        "mkdir -p /temporal",
        "make bins",
        "mv temporal-server temporal-sql-tool tdbg /temporal/.",
        "wget -O - https://github.com/temporalio/cli/releases/download/v0.10.7/temporal_cli_0.10.7_linux_amd64.tar.gz | tar xzf - -C /temporal",
        "wget -O - https://github.com/jwilder/dockerize/releases/download/v0.7.0/dockerize-linux-amd64-v0.7.0.tar.gz | tar xzf - -C /temporal",
    ],
    extract_file = "/temporal",
    image = ":temporal-srcs.tar",
)

# Pack schema files into proper foler with proper permissions
pkg_tar(
    name = "schemas",
    srcs = glob([
        "schema/postgresql/v96/**/*.sql",
        "schema/postgresql/v96/**/*.json",
    ]),
    mode = "0755",
    package_dir = "/temporal",
    strip_prefix = ".",  # Preserve the folder structure.
)

# The built temporal binaries and artifacts
pkg_tar(
    name = "temporal",
    srcs = [
        ":docker/config_template.yaml",
        ":temporal-bins/temporal",
    ],
    package_dir = "/etc",
    visibility = ["//visibility:public"],
    deps = [
        ":schemas",
    ],
)
