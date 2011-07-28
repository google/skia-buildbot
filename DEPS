use_relative_paths = True

deps = {
  # Chromium buildbot code and its dependencies
  "third_party/chromium_buildbot": "http://src.chromium.org/svn/trunk/tools/build@94518",
  "third_party/depot_tools": "http://src.chromium.org/svn/trunk/tools/depot_tools",
}

hooks = [
  {
    "pattern": ".",
    "action": ["python", "buildbot/hooks.py"],
  },
]
