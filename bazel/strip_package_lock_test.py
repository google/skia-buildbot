"""Unit tests for strip-package-lock.py."""

import importlib.util
import os
import unittest


def _load_module():
    path = os.path.join(os.path.dirname(__file__), "strip-package-lock.py")
    spec = importlib.util.spec_from_file_location("strip_package_lock", path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


strip_mod = _load_module()


class PromoteTargetNestedDepsTest(unittest.TestCase):

    def test_no_nested_deps_returns_unchanged(self):
        pkgs = {
            "node_modules/target": {"version": "1.0.0", "dependencies": {"foo": "^1.0.0"}},
            "node_modules/foo": {"version": "1.2.3"},
        }
        got = strip_mod.promote_target_nested_deps(pkgs, "target")
        self.assertEqual(got, pkgs)

    def test_simple_displacement_negotiator_compression_shape(self):
        pkgs = {
            "node_modules/verdaccio": {
                "version": "7.1.2",
                "dependencies": {"compression": "^1.8.2", "negotiator": "^1.1.0"},
            },
            "node_modules/verdaccio/node_modules/negotiator": {"version": "1.1.0"},
            "node_modules/compression": {
                "version": "1.8.2",
                "dependencies": {"negotiator": "~0.6.4"},
            },
            "node_modules/negotiator": {"version": "0.6.4"},
        }
        got = strip_mod.promote_target_nested_deps(pkgs, "verdaccio")
        self.assertEqual(got["node_modules/negotiator"]["version"], "1.1.0")
        self.assertEqual(
            got["node_modules/compression/node_modules/negotiator"]["version"],
            "0.6.4",
        )
        self.assertNotIn("node_modules/verdaccio/node_modules/negotiator", got)

    def test_displaced_package_preserves_its_own_nested_subtree(self):
        pkgs = {
            "node_modules/target": {
                "version": "2.0.0",
                "dependencies": {"user": "^1.0.0", "lib": "^2.0.0"},
            },
            "node_modules/target/node_modules/lib": {"version": "2.0.0"},
            "node_modules/user": {"version": "1.0.0", "dependencies": {"lib": "^1.0.0"}},
            "node_modules/lib": {"version": "1.0.0", "dependencies": {"sub": "^1.0.0"}},
            "node_modules/lib/node_modules/sub": {"version": "1.5.0"},
        }
        got = strip_mod.promote_target_nested_deps(pkgs, "target")
        self.assertEqual(got["node_modules/lib"]["version"], "2.0.0")
        self.assertNotIn("node_modules/lib/node_modules/sub", got)
        self.assertEqual(got["node_modules/user/node_modules/lib"]["version"], "1.0.0")
        self.assertEqual(
            got["node_modules/user/node_modules/lib/node_modules/sub"]["version"],
            "1.5.0",
        )

    def test_scoped_packages_nested_and_displaced(self):
        pkgs = {
            "node_modules/target": {
                "version": "2.0.0",
                "dependencies": {"@scope/user": "^1.0.0", "@scope/lib": "^2.0.0"},
            },
            "node_modules/target/node_modules/@scope/lib": {"version": "2.0.0"},
            "node_modules/@scope/user": {
                "version": "1.0.0",
                "dependencies": {"@scope/lib": "^1.0.0"},
            },
            "node_modules/@scope/lib": {"version": "1.0.0"},
        }
        got = strip_mod.promote_target_nested_deps(pkgs, "target")
        self.assertEqual(got["node_modules/@scope/lib"]["version"], "2.0.0")
        self.assertEqual(
            got["node_modules/@scope/user/node_modules/@scope/lib"]["version"],
            "1.0.0",
        )
        self.assertNotIn("node_modules/target/node_modules/@scope/lib", got)

    def test_dependent_with_closer_nested_copy_is_not_overwritten(self):
        pkgs = {
            "node_modules/target": {
                "version": "2.0.0",
                "dependencies": {"a": "^1.0.0", "b": "^1.0.0", "lib": "^2.0.0"},
            },
            "node_modules/target/node_modules/lib": {"version": "2.0.0"},
            "node_modules/a": {"version": "1.0.0", "dependencies": {"lib": "^0.5.0"}},
            "node_modules/a/node_modules/lib": {"version": "0.5.0"},
            "node_modules/b": {"version": "1.0.0", "dependencies": {"lib": "^1.0.0"}},
            "node_modules/lib": {"version": "1.0.0"},
        }
        got = strip_mod.promote_target_nested_deps(pkgs, "target")
        self.assertEqual(got["node_modules/lib"]["version"], "2.0.0")
        # `a` already had a closer nested copy at 0.5.0, which must remain intact.
        self.assertEqual(got["node_modules/a/node_modules/lib"]["version"], "0.5.0")
        # `b` resolved to top-level 1.0.0, so it gets the displaced copy.
        self.assertEqual(got["node_modules/b/node_modules/lib"]["version"], "1.0.0")

    def test_chained_displacement(self):
        # target has nested a@2, b@2, c@2; top-level user -> a@1 -> b@1 -> c@1.
        pkgs = {
            "node_modules/target": {
                "version": "2.0.0",
                "dependencies": {"user": "^1.0.0", "a": "^2.0.0", "b": "^2.0.0", "c": "^2.0.0"},
            },
            "node_modules/target/node_modules/a": {"version": "2.0.0"},
            "node_modules/target/node_modules/b": {"version": "2.0.0"},
            "node_modules/target/node_modules/c": {"version": "2.0.0"},
            "node_modules/user": {"version": "1.0.0", "dependencies": {"a": "^1.0.0"}},
            "node_modules/a": {"version": "1.0.0", "dependencies": {"b": "^1.0.0"}},
            "node_modules/b": {"version": "1.0.0", "dependencies": {"c": "^1.0.0"}},
            "node_modules/c": {"version": "1.0.0"},
        }
        got = strip_mod.promote_target_nested_deps(pkgs, "target")
        self.assertEqual(got["node_modules/a"]["version"], "2.0.0")
        self.assertEqual(got["node_modules/b"]["version"], "2.0.0")
        self.assertEqual(got["node_modules/c"]["version"], "2.0.0")
        self.assertEqual(got["node_modules/user/node_modules/a"]["version"], "1.0.0")
        self.assertEqual(
            got["node_modules/user/node_modules/a/node_modules/b"]["version"],
            "1.0.0",
        )
        self.assertEqual(
            got["node_modules/user/node_modules/a/node_modules/b/node_modules/c"]["version"],
            "1.0.0",
        )

    def test_peer_dependency_on_promoted_package(self):
        # express-rate-limit shape: express@5 is nested under verdaccio and
        # absent at top level; promoting it satisfies express-rate-limit's peer.
        pkgs = {
            "node_modules/verdaccio": {
                "version": "7.1.2",
                "dependencies": {"express": "^5.3.0", "express-rate-limit": "^8.7.1"},
            },
            "node_modules/verdaccio/node_modules/express": {"version": "5.3.0"},
            "node_modules/express-rate-limit": {
                "version": "8.7.1",
                "peerDependencies": {"express": ">=4.11"},
            },
        }
        got = strip_mod.promote_target_nested_deps(pkgs, "verdaccio")
        self.assertEqual(got["node_modules/express"]["version"], "5.3.0")
        self.assertNotIn("node_modules/verdaccio/node_modules/express", got)
        self.assertNotIn("node_modules/express-rate-limit/node_modules/express", got)


if __name__ == "__main__":
    unittest.main()
