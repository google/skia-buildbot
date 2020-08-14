package(default_visibility=["//visibility:public"])

load("@build_bazel_rules_nodejs//internal/npm_install:npm_umd_bundle.bzl", "npm_umd_bundle")

##############################
# UMD bundles for common-sk. #
##############################

COMMON_SK_MODULES = [
    "dom",
    "hintable",
    "human",
    "jsonOrThrow",
    "object",
    "query",
    "stateReflector",
]

filegroup(
    name = "common-sk_umd",
    srcs = ["common-sk/modules/%s.umd.js" % module for module in COMMON_SK_MODULES],
)

[
    npm_umd_bundle(
        name = "common-sk_%s_umd" % module,
        package_name = "common-sk/modules/%s" % module,
        entry_point = "@infra-sk_npm//:node_modules/common-sk/modules/%s.js" % module,
        package = "@infra-sk_npm//common-sk",
    )
    for module in COMMON_SK_MODULES
]

######################
# Other UMD bundles. #
######################

npm_umd_bundle(
    name = "chai_umd",
    package_name = "chai",
    entry_point = "@infra-sk_npm//:node_modules/chai/lib/chai.js",
    package = "@infra-sk_npm//chai",
)

npm_umd_bundle(
    name = "sinon_umd",
    package_name = "sinon",
    entry_point = "@infra-sk_npm//:node_modules/sinon/lib/sinon.js",
    package = "@infra-sk_npm//sinon",
)
