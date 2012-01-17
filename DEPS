# This DEPS file checks out a read-only copy of the Chromium buildbot code at
# a revision that is known to work with the current Skia buildbot setup.
#
# Thus, this DEPS file is useful for:
#  - users who just want to run, not modify, the Skia buildbot
#  - developers who wish to make changes to the Skia buildbot config but NOT
#    the underlying Chromium buildbot code
#
# To check out the Skia buildbot code using this DEPS file, run:
#  gclient config https://skia.googlecode.com/svn/buildbot
#  gclient sync

use_relative_paths = True

vars = {
  "chromium_trunk": "http://src.chromium.org/svn/trunk",
  "chromium_revision": "117902",
}

deps = {
  # Chromium buildbot code.
  "third_party/chromium_buildbot":
    Var("chromium_trunk") + "/tools/build@" + Var("chromium_revision"),
  "third_party/chromium_buildbot/scripts/command_wrapper/bin":
    Var("chromium_trunk") + "/tools/command_wrapper/bin@" + Var("chromium_revision"),
  "third_party/depot_tools":
    Var("chromium_trunk") + "/tools/depot_tools@" + Var("chromium_revision"),

  # Dependencies of the Chromium buildbot code.
  # I tried to use From() to link to Chromium's /tools/build/DEPS dependencies,
  # but I couldn't get it to work... so I have hard-coded these dependencies.
  "third_party/chromium_buildbot/third_party/gsutil":
    "svn://svn.chromium.org/gsutil/trunk/src@145",
  "third_party/chromium_buildbot/third_party/gsutil/boto":
    "svn://svn.chromium.org/boto@3",

  # Also, each slave needs its own scripts/slave directory alongside
  # (due to hard-coded paths in the buildbot, argh)
  "scripts/slave":
    Var("chromium_trunk") + "/tools/build/scripts/slave@" + Var("chromium_revision"),
  "configs/chromium/scripts/slave":
    Var("chromium_trunk") + "/tools/build/scripts/slave@" + Var("chromium_revision"),
}

hooks = [
  {
    "pattern": ".",
    "action": ["python", "buildbot/hooks.py"],
  },
]
