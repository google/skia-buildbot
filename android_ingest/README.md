android_ingest
==============

An application that ingests incoming data from the Android performance tests
and creates JSON files in Google Cloud Storage that can be ingested by Perf.
It additionally created a git repo that mirrors each buildid into a Git
repository, which is what Perf is expecting.

Example Input JSON
------------------

The incoming test data has the form:

{
  "build_id":"3567162",
  "build_flavor":"marlin-userdebug",
  "metrics": {
    "android.platform.systemui.tests.jank.LauncherJankTests#testOpenAllAppsContainer":
      "{
      gfx-avg-slow-ui-thread=4.417545616725611,
      gfx-max-slow-bitmap-uploads=0.0,
      gfx-max-frame-time-95=28,
      gfx-max-frame-time-50=7,
      gfx-max-slow-ui-thread=5.056179775280898,
      gfx-avg-frame-time-50=6.6,
      gfx-max-jank=7.58,
      gfx-avg-slow-draw=1.676942818691121,
      gfx-avg-frame-time-95=24.2,
      gfx-max-frame-time-90=14,
      gfx-avg-frame-time-90=12.2,
      gfx-avg-jank=6.986,
      gfx-max-missed-vsync=4.735376044568245,
      gfx-avg-slow-bitmap-uploads=0.0,
      gfx-max-high-input-latency=0.0,
      gfx-max-frame-time-99=57,
      gfx-avg-missed-vsync=3.969515821101061,
      gfx-avg-frame-time-99=45.8,
      gfx-max-slow-draw=2.785515320334262,
      gfx-avg-high-input-latency=0.0
      }"
    },
  "branch":"google-marlin-marlin-O"
}
