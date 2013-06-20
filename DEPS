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
  "chromium_revision": "179720",
  "depot_tools_revision": "015fd3d953081bb86f6e7b4c8788283f370d5df8s",
  "telemetry_chromium_revision": "206068",
  "webpagereplay_revision": "511",
}

deps = {
  # Chromium trunk code for run_skpicture_printer.
  "third_party/chromium_trunk/tools/perf":
    Var("chromium_trunk") + "/src/tools/perf@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/tools/telemetry":
    Var("chromium_trunk") + "/src/tools/telemetry@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/chrome/test/functional":
    Var("chromium_trunk") + "/src/chrome/test/functional@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/build/android/pylib":
    Var("chromium_trunk") + "/src/build/android/pylib@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/tools/crx_id":
    Var("chromium_trunk") + "/src/tools/crx_id@" + Var("telemetry_chromium_revision"),

  # build/android/pylib/android_commands.py requires android_testrunner to be in third_party.
  "third_party/chromium_trunk/third_party/android_testrunner":
    Var("chromium_trunk") + "/src/third_party/android_testrunner@" + Var("chromium_revision"),
  
  # chrome_remote_control/replay_server.py requires webpagereplay to be in src/third_party.
  "third_party/src/third_party/webpagereplay":
    "http://web-page-replay.googlecode.com/svn/trunk/@" + Var("webpagereplay_revision"),
  
  # Chromium buildbot code.
  "third_party/chromium_buildbot":
    Var("chromium_trunk") + "/tools/build@" + Var("chromium_revision"),
  "third_party/chromium_buildbot/scripts/command_wrapper/bin":
    Var("chromium_trunk") + "/tools/command_wrapper/bin@" + Var("chromium_revision"),
  "third_party/depot_tools":
    "https://chromium.googlesource.com/chromium/tools/depot_tools.git@" + Var("depot_tools_revision"),

  # Dependencies of the Chromium buildbot code.
  # I tried to use From() to link to Chromium's /tools/build/DEPS dependencies,
  # but I couldn't get it to work... so I have hard-coded these dependencies.
  "third_party/chromium_buildbot/third_party/gsutil":
    "svn://svn.chromium.org/gsutil/trunk/src@261",
  "third_party/chromium_buildbot/third_party/gsutil/boto":
    "svn://svn.chromium.org/boto@7",
}

hooks = [
  {
    "pattern": ".",
    "action": ["python", "buildbot/hooks.py"],
  },
]
