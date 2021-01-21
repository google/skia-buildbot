"""This module defines a custom go_test macro that produces separate targets for manual tests."""

load("@io_bazel_rules_go//go:def.bzl", _go_test = "go_test")

def go_test(name, srcs, tags = [], **kwargs):
    """Wrapper around rules_go's go_test rule which generates a separate target for manual tests.

    The purpose of this macro is to automatically create separate test targets for manual Go tests,
    which will be tagged as manual. This excludes any such targets from Bazel wildcards, e.g.
    "bazel test ...". In order to run manual tests, one must target them explicitly, e.g.
    "bazel test //path/to:my_manual_test".

    This macro is meant to be used with the gazelle:map_kind directive, see
    https://github.com/bazelbuild/bazel-gazelle#directives.

    If srcs does not contain any files ending in "_manual_test.go", this macro will simply call
    rules_go's go_test rule with the given arguemnts, producing a single test target.

    If srcs contains one or more files ending in "_manual_test.go", this macro will call rules_go's
    go_test rule twice: one with the given target name and all srcs *except* the manual tests, and
    another one with just the manual tests. The name passed to the second go_test call will be the
    result of replacing the "_test" suffix in the given name with "_manual_test". The target for
    manual tests will be tagged as manual.

    Any other arguments will be copied into both go_test targets verbatim.

    Example:

        # Macro invocation.

        go_test(
            name = "foo_test",
            srcs = [
                "foo_test.go",
                "foo_manual_test.go",
            ],
            ...
        )

        # Resulting rules_go's go_test targets:

        go_test(
            name = "foo_test",
            srcs = ["foo_test.go"],
            ...
        )

        go_test(
            name = "foo_manual_test",
            srcs = ["foo_manual_test.go"],
            tags = ["manual"],
            ...
        )

    Args:
        name: Base name of the target(s) to generate.
        srcs: Any *_test.go files.
        tags: Any tags for the resulting go_test targets.
        **kwargs: Any other arguments to pass to the resulting go_test targets.
    """

    # Gazelle only includes files ending with _test.go in the srcs attribute of go_test targets.
    # Fail loudly if this ceases to be true for any reason.
    for src in srcs:
        if not src.endswith("_test.go"):
            fail("go_test called with a src file that does not end with \"_test.go\".")

    manual_tests = [src for src in srcs if src.endswith("_manual_test.go")]
    non_manual_tests = [src for src in srcs if not src in manual_tests]

    _go_test(name = name, srcs = non_manual_tests, tags = tags, **kwargs)

    if manual_tests:
        _go_test(
            name = name[:-4] + "manual_test",  # Turn e.g. foo_test into foo_manual_test.
            srcs = manual_tests,
            tags = tags + ["manual"],
            **kwargs
        )
