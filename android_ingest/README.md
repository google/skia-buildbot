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
	"build_id": "3567162",
	"build_flavor": "marlin-userdebug",
	"metrics": {
		"android.platform.systemui.tests.jank.LauncherJankTests#testAppSwitchGMailtoHome": {
			"frame-fps": "9.328892269753897",
			"frame-avg-jank": "8.4",
			"frame-max-frame-duration": "7.834711093388444",
			"frame-max-jank": "10"
		},
    ...
	},
	"branch": "google-marlin-marlin-O"
}
