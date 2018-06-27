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

Initializing the target repo
============================

To start with a new repo it must be initialized correctly, by commiting
a BUILDID file that contains the initial buildid and timestamp that
the repo should start from, with a correct subject. For example, you could
populate BUILDID with the buildid 3529135, which has a timestamp of 1480456484.
This means populating the BUILDID file with:

   3529135 1480456484

Then adding that file to the repo:

   git add BUILDID

Then commit with a subject message that is the redirector URL, i.e.
append the buildid to "https://android-ingest.skia.org/r/", and use the
flag --date and the environment variable GIT_COMMITTER_DATE to set
both the author and commiter date to the matching timestamp.

   GIT_COMMITTER_DATE=1480456484 git commit -m "https://android-ingest.skia.org/r/3529135" --date=1480456484

Upload Log
==========

POST requests with new data are not able to be re-triggered so we have to take
special care not to lose data. A transaction log is kept of all incoming POST
requests that can be replayed if needed.

Use the ./replay-log.sh script to replay logs back into android_ingest. See
the comments in the script for how to run it.
