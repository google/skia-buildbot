# This DEPS file checks out a read-only copy of the Chromium buildbot code at
# a revision that is known to work with the current Skia buildbot setup.
#
# Thus, this DEPS file is useful for:
#  - users who just want to run, not modify, the Skia buildbot
#  - developers who wish to make changes to the Skia buildbot config but NOT
#    the underlying Chromium buildbot code
#
# To check out the Skia buildbot code using this DEPS file, run:
#  gclient config https://skia.googlesource.com/buildbot.git
#  gclient sync

use_relative_paths = True

vars = {
  "chromium_trunk": "http://src.chromium.org/svn/trunk",
  "chromium_revision": "179720",
  "telemetry_chromium_revision": "253293",
  "webpagereplay_revision": "540",
  "telemetry_webkit_trunk": "http://src.chromium.org/blink/trunk",
  "telemetry_webkit_revision": "167843"
}

deps = {
  # Chromium trunk code for running telemetry binaries.
  "third_party/chromium_trunk/src/tools/perf":
    Var("chromium_trunk") + "/src/tools/perf@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/src/tools/telemetry":
    Var("chromium_trunk") + "/src/tools/telemetry@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/src/chrome/test/functional":
    Var("chromium_trunk") + "/src/chrome/test/functional@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/src/build/android/pylib":
    Var("chromium_trunk") + "/src/build/android/pylib@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/src/tools/crx_id":
    Var("chromium_trunk") + "/src/tools/crx_id@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/src/build/util":
    Var("chromium_trunk") + "/src/build/util@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/src/third_party/flot":
    Var("chromium_trunk") + "/src/third_party/flot@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/src/third_party/WebKit/PerformanceTests/resources":
    Var("telemetry_webkit_trunk") + "/PerformanceTests/resources@" + Var("telemetry_webkit_revision"),
  "third_party/chromium_trunk/src/third_party/webpagereplay":
    "http://web-page-replay.googlecode.com/svn/trunk/@" + Var("webpagereplay_revision"),

  # build/android/pylib/android_commands.py requires android_testrunner to be in third_party.
  "third_party/chromium_trunk/src/third_party/android_testrunner":
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
    Var("chromium_trunk") + "/tools/depot_tools",

  # Dependencies of the Chromium buildbot code.
  # I tried to use From() to link to Chromium's /tools/build/DEPS dependencies,
  # but I couldn't get it to work... so I have hard-coded these dependencies.
  "third_party/chromium_buildbot/third_party/gsutil":
    "svn://svn.chromium.org/gsutil/trunk/src@261",
  "third_party/chromium_buildbot/third_party/gsutil/boto":
    "svn://svn.chromium.org/boto@7",
}

