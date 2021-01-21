"""This module defines a custom go_test macro that produces separate targets for manual tests."""

load("@io_bazel_rules_go//go:def.bzl", _go_test = "go_test")

def go_test(name, **kwargs):
    """Wrapper around rules_go's go_test rule which generates a separate target for manual tests.

    This is currently not implemented and simply calls rules_go's go_test rule with the given
    arguments.

    TODO(lovisolo): Implement.

    Args:
        name: Name of the target.
        **kwargs: Any other arguments to pass to the resulting go_test targets.
    """
    _go_test(name = name, **kwargs)
