AutoRoll Production Manual
==========================

General information about the AutoRoller is available in the
[README](./README.md).

The AutoRoller requires that gitcookies are in metadata under the key gitcookies_$INSTANCE_NAME.

Alerts
======

autoroll_failed
---------------

The most recent DEPS roll attempt failed. This is usually due to a change in the
child repo which is incompatible with the parent and requires some investigation
into which bots failed and why. Fixing this usually requires a commit to the
child repo, either a revert or a fix. This alert is only enabled for Skia.


no_rolls_24h
------------

There have been no successful rolls landed in the last 24 hours. This alert
assumes that at least one commit has landed in the child repo in the last 24
hours; if that is not the case, then this alert can be safely ignored. This
alert is only enabled for Skia.


http_latency
------------

The frontend server is taking too long to respond. Check the logs for any
obvious cause; if load is high, we may need to bring up more replicas.


error_rate
----------

The AutoRoll server on the given host is logging errors at a higher-than-normal
rate. This warrants investigation in the logs. The most common cause of this
alert is transient errors while communicating with Git or Gerrit. In that case,
there is not much which can be done other than to silence the alert and hope
that things improve. If the failure persists, contact the current on-call for
the Git service.

If this happened on the Skia->Flutter roller then also take a look at the
flutter_license_script_failure section below.


flutter_license_script_failure
------------------------------

The Skia->Flutter roller has failed due to errors from Flutter's license script.
Inform the [Skia Gardener](https://rotations.corp.google.com/rotation/4699606003744768) that the
Skia->Flutter roller is failing the license script and that you are investigating.

Take a look at the cloud logs of the roller [here](https://console.cloud.google.com/logs/viewer?project=skia-public&advancedFilter=logName%3D%22projects%2Fskia-public%2Flogs%2Fautoroll-be-skia-flutter-autoroll%22).
Failures due to license script errors typically look like this:
"Failed to transition from "idle" to "active": Error when running pre-upload step: Error when running dart license script: Command exited with exit status 1:"...

If the license script error looks unrelated to Skia ([example](https://github.com/flutter/flutter/issues/25679)),
then file a bug to the Flutter team via [Github issues](https://github.com/flutter/flutter/issues/new/choose)
with log snippets. Informing liyuqian@ about the issue might expedite the fix.

If the error looks related to Skia, then take a look at the recent unrolled
changes to see if you can identify which change caused the license script to
fail. Sometimes adding a new directory in third_party without a LICENSE file
can cause the script to fail ([example](https://bugs.chromium.org/p/skia/issues/detail?id=8027)).
Sometimes a typo in license headers can cause the script to fail ([example](https://skia-review.googlesource.com/c/skia/+/241879)).

If you need to run the license scripts manually on a clean local checkout,
then follow these steps-
* Checkout the necessary repos:
  * git clone https://github.com/flutter/engine
  * cd engine
  * echo """
solutions = [
  {
    'managed': False,
    'name': 'src/flutter',
    'url': 'git@github.com:flutter/engine.git',
    'custom_deps': {},
    'deps_file': 'DEPS',
    'safesync_url': '',
  },
]
""" > .gclient
  * gclient sync
* cd src/flutter
* Change the Skia rev in the DEPS here if necessary.
* Run the license scripts:
  * cd tools/licenses
  * ../../../third_party/dart/tools/sdks/dart-sdk/bin/dart pub get
  * Run dart license script to create new licenses.
    * rm -rf ../../../out/licenses
    * ../../../third_party/dart/tools/sdks/dart-sdk/bin/dart lib/main.dart --src ../../.. --out ../../../out/licenses --golden ../../ci/licenses_golden
  * Copy from out dir to goldens dir. This is required for updating the release file in sky_engine/LICENSE.
    * cp ../../../out/licenses/licenses_skia ../../ci/licenses_golden/licenses_skia
  * Update ../../sky/packages/sky_engine/LICENSE using the dart license script.
    * ../../../third_party/dart/tools/sdks/dart-sdk/bin/dart lib/main.dart --release --src ../../.. --out ../../../out/licenses

Useful documentation links:
* How to checkout flutter is documented [here](https://github.com/flutter/flutter/wiki/Setting-up-the-Engine-development-environment).
* License script documentation is [here](https://github.com/flutter/engine/blob/master/tools/licenses/README.md).
* The code for the pre-upload license step used by the autoroller is [here](https://skia.googlesource.com/buildbot/+show/main/autoroll/go/repo_manager/parent/pre_upload_steps.go).


Other Troubleshooting Tips
--------------------------

#### The autoroll page displays "Status: error" and doesn't upload CLs ####

Some issue is preventing the roller from running normally. You'll need to look
through the logs for more information. Normally this is transient, but it can
also be caused by mis-configuration of the roller, eg. the configured reviewer
is not a committer.


#### Something is wrong, and I need to shut down the roller ASAP! ####

Setting the roller mode to "Stopped" should be enough in most cases. If not,
you can use `kubectl delete -f <file>` to kill it.
