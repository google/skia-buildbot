"""This module defines the karma_test rule."""

load("@npm//:karma/package_json.bzl", _karma_test_bin = "bin")
load("//infra-sk:ts_library.bzl", "ts_library")
load("//infra-sk/esbuild:esbuild.bzl", "esbuild_dev_bundle")

def karma_test(
        name,
        src,
        deps = [],
        karma_config_file = "//infra-sk/karma_test:karma_config",
        static_karma_files = []):
    """Runs TypeScript unit tests in a browser with Karma, using Mocha as the test runner.

    When invoked via `bazel test`, a headless Chrome browser will be used. This supports testing
    multiple karma_test targets in parallel, and works on RBE.

    When invoked via `bazel run`, it prints out a URL to stdout that can be opened in the browser,
    e.g. to debug the tests using the browser's developer tools. Source maps are generated.

    When invoked via `ibazel test`, the test runner never exits, and tests will be rerun every time
    a source file is changed.

    When invoked via `ibazel run`, it will act the same way as `bazel run`, but the tests will be
    rebuilt automatically when a source file changes. Reload the browser page to see the changes.

    Args:
      name: The name of the target.
      src: A single TypeScript source file.
      deps: Any ts_library dependencies.
      karma_config_file: A string that refers to the karma.conf.js file which should be used to run
         the test. If omitted, a sensible default is used.
      static_karma_files: A list of labels for additional files that should be made available to
         be served statically for karma tests. If this is provided, a custom karma_config_file
         should also be supplied to make use of the absolute file paths which are appended to the
         karma "start" command.
    """

    # Enforce test naming conventions. The Gazelle extension for front-end code relies on these.
    if not src.endswith("_test.ts"):
        fail("Karma tests must end with \"_test.ts\".")
    for suffix in ["_puppeteer_test.ts", "_nodejs_test.ts"]:
        if src.endswith(suffix):
            fail("Karma tests cannot end with \"%s\"." % suffix)

    ts_library(
        name = name + "_lib",
        srcs = [src],
        deps = deps,
    )

    esbuild_dev_bundle(
        name = name + "_bundle",
        entry_point = src,
        deps = [name + "_lib"],
        output = name + "_bundle.js",
    )

    static_template_args = []
    for f in static_karma_files:
        # Include absolute file paths to the static Karma files.
        static_template_args.append("$(location %s)" % f)

    # See https://docs.aspect.build/rulesets/aspect_rules_js/docs/#using-binaries-published-to-npm.
    _karma_test_bin.karma_test(
        name = name,
        size = "large",
        data = [
            name + "_bundle.js",
            karma_config_file,
            "//:node_modules/karma-chrome-launcher",
            "//:node_modules/karma-sinon",
            "//:node_modules/karma-mocha",
            "//:node_modules/karma-chai",
            "//:node_modules/karma-chai-dom",
            "//:node_modules/karma-spec-reporter",
            "//:node_modules/mocha",
            "@rules_browsers//browsers/chromium",
        ] + static_karma_files,
        args = [
            "start",
            "$(location %s)" % karma_config_file,
            "$(location %s_bundle.js)" % name,
        ] + static_template_args,
        tags = [
            # Necessary for it to work with ibazel.
            "ibazel_notify_changes",
        ],
        env = {
            "CHROME_BIN": "$(CHROME-HEADLESS-SHELL)",
        },
        toolchains = ["@rules_browsers//browsers/chromium:toolchain_alias"],
    )
