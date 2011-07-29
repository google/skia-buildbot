use_relative_paths = True

vars = {
  "chromium_trunk": "http://src.chromium.org/svn/trunk",
  "chromium_revision": "94665",
}

deps = {
  # Chromium buildbot code and its dependencies
  "third_party/chromium_buildbot":
    Var("chromium_trunk") + "/tools/build@" + Var("chromium_revision"),
  "third_party/depot_tools":
    Var("chromium_trunk") + "/tools/depot_tools@" + Var("chromium_revision"),
}

hooks = [
  {
    "pattern": ".",
    "action": ["python", "buildbot/hooks.py"],
  },
]
