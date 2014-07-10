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
  "chromium_git": "https://chromium.googlesource.com",
  "skia_git": "https://skia.googlesource.com",
  "telemetry_chromium_revision": "278114",
  "webpagereplay_revision": "546",
  "telemetry_webkit_trunk": "http://src.chromium.org/blink/trunk",
  "telemetry_webkit_revision": "176408"
}

deps = {
  # Utilities shared between the Skia and Skia-Buildbot repositories.
  "common":
    Var("skia_git") + "/common.git@c92e6d8058240b0804b28fdc4f78261b7133431d",

  # Chromium trunk code for running telemetry binaries.
  "third_party/chromium_trunk/src/tools/perf":
    Var("chromium_trunk") + "/src/tools/perf@" + Var("telemetry_chromium_revision"),
  "third_party/chromium_trunk/src/tools/telemetry":
    Var("chromium_trunk") + "/src/tools/telemetry@" + Var("telemetry_chromium_revision"),
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

  # Chromium buildbot code, pinned at an old revision for compatibility with our
  # buildbot code.
  "third_party/chromium_buildbot":
    Var("chromium_trunk") + "/tools/build@" + Var("chromium_revision"),
  "third_party/chromium_buildbot/scripts/command_wrapper/bin":
    Var("chromium_git") + "/chromium/tools/command_wrapper/bin.git@2eeebba9a512cae9e4e9312f5ec728dbdad80bd0",
  "third_party/depot_tools":
    Var("chromium_git") + "/chromium/tools/depot_tools.git",

  # Tip-of-tree Chromium buildbot code.
  "third_party/chromium_buildbot_tot":
    Var("chromium_git") + "/chromium/tools/build.git",

  # Dependencies of the Chromium buildbot code.
  # I tried to use From() to link to Chromium's /tools/build/DEPS dependencies,
  # but I couldn't get it to work... so I have hard-coded these dependencies.
  "third_party/chromium_buildbot/third_party/gsutil":
    Var("chromium_git") + "/external/gsutil/src.git@b41305d0b538bae46777e1d9562ecec0149f8d44",
  "third_party/chromium_buildbot/third_party/gsutil/boto":
    Var("chromium_git") + "/external/boto.git@98fc59a5896f4ea990a4d527548204fed8f06c64",
}

