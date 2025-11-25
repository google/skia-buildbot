"""Tests for owners_layers.bzl"""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load(":owners_layers.bzl", "ROOT_UID_GID", "SKIA_UID_GID", "get_fixup_owners_layers")

_OTHER_UID_GID = "5000.5000"

def _simple_test_impl(ctx):
    env = unittest.begin(ctx)
    dirs = ["/usr/bin"]
    owners = {}
    layers = get_fixup_owners_layers(dirs, owners)
    asserts.equals(env, 1, len(layers))
    asserts.equals(env, ROOT_UID_GID, layers[0].owner)
    asserts.equals(env, ["/", "/usr", "/usr/bin"], layers[0].paths)
    return unittest.end(env)

_simple_test = unittest.make(_simple_test_impl)

def _non_root_user_test_impl(ctx):
    env = unittest.begin(ctx)
    dirs = ["/usr/bin", "/home/skia/somedir"]
    owners = {
        "/home/skia": SKIA_UID_GID,
    }
    layers = get_fixup_owners_layers(dirs, owners)
    asserts.equals(env, 2, len(layers))
    asserts.equals(env, SKIA_UID_GID, layers[0].owner)
    asserts.equals(env, ["/home/skia", "/home/skia/somedir"], layers[0].paths)
    asserts.equals(env, ROOT_UID_GID, layers[1].owner)
    asserts.equals(env, ["/", "/home", "/usr", "/usr/bin"], layers[1].paths)
    return unittest.end(env)

_non_root_user_test = unittest.make(_non_root_user_test_impl)

def _nested_user_test_impl(ctx):
    env = unittest.begin(ctx)
    dirs = [
        "/usr/bin",
        "/home/skia/somedir",
        "/home/skia/nested/user/dir",
    ]
    owners = {
        "/home/skia": SKIA_UID_GID,
        "/home/skia/nested/user": _OTHER_UID_GID,
    }
    layers = get_fixup_owners_layers(dirs, owners)
    asserts.equals(env, 3, len(layers))
    asserts.equals(env, _OTHER_UID_GID, layers[0].owner)
    asserts.equals(env, ["/home/skia/nested/user", "/home/skia/nested/user/dir"], layers[0].paths)
    asserts.equals(env, SKIA_UID_GID, layers[1].owner)
    asserts.equals(env, ["/home/skia", "/home/skia/nested", "/home/skia/somedir"], layers[1].paths)
    asserts.equals(env, ROOT_UID_GID, layers[2].owner)
    asserts.equals(env, ["/", "/home", "/usr", "/usr/bin"], layers[2].paths)
    return unittest.end(env)

_nested_user_test = unittest.make(_nested_user_test_impl)

def _trailing_slash_test_impl(ctx):
    env = unittest.begin(ctx)
    dirs = ["/", "/usr/bin/"]
    owners = {}
    layers = get_fixup_owners_layers(dirs, owners)
    asserts.equals(env, 1, len(layers))
    asserts.equals(env, ROOT_UID_GID, layers[0].owner)
    asserts.equals(env, ["/", "/usr", "/usr/bin"], layers[0].paths)
    return unittest.end(env)

_trailing_slash_test = unittest.make(_trailing_slash_test_impl)

def get_fixup_owners_layers_test_suite(name):
    unittest.suite(
        name,
        _simple_test,
        _non_root_user_test,
        _nested_user_test,
        _trailing_slash_test,
    )
