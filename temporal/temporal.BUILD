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
        "go build -o /temporal/temporal-server ./cmd/server",
        "go build -o /temporal/tdbg ./cmd/tools/tdbg",
        "go build -o /temporal/temporal-sql-tool ./cmd/tools/sql",
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
