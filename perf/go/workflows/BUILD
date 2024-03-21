load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "workflows",
    srcs = ["workflows.go"],
    importpath = "go.skia.org/infra/perf/go/workflows",
    visibility = ["//visibility:public"],
    deps = ["//perf/go/culprit/proto/v1"],
)
