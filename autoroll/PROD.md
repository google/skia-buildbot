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

One of the AutoRoll servers is taking too long to respond. The name of the
prober which triggered the alert should indicate which roller is slow.


error_rate
----------

The AutoRoll server on the given host is logging errors at a higher-than-normal
rate. This warrants investigation in the logs.

The state machine may throw errors like this: "Transition is already in
progress; did a previous transition get interrupted?"  That is intended to
detect the case where we interrupted the process during a state transition, and
we may be in an undefined state. This requires manual investigation, after which
you should remove the /mnt/pd0/autoroll_workdir/state_machine_transitioning
file. This error may also prevent the roller from starting up, which is by
design.

If this happened on the Skia->Flutter roller then also take a look at the
flutter_license_script_failure section below.


flutter_license_script_failure
------------------------------

The Skia->Flutter roller has failed due to errors from Flutter's license script.
Take a look at the cloud logs of the roller [here](https://pantheon.corp.google.com/logs/viewer?project=skia-public&advancedFilter=logName%3D%22projects%2Fskia-public%2Flogs%2Fautoroll-be-skia-flutter-autoroll%22).
Failures due to license script errors typically look like this:
"Failed to transition from "idle" to "active": Error when running pre-upload step: Error when running dart license script: Command exited with exit status 1:"...

If the license script error looks unrelated to Skia ([example](https://github.com/flutter/flutter/issues/25679)),
then file a bug to the Flutter team via [Github issues](https://github.com/flutter/flutter/issues/new/choose)
with log snippets. Informing liyuqian@ about the issue might expedite the fix.

If the error looks related to Skia, then take a look at the recent unrolled
changes to see if you can identify which change caused the license script to
fail. Sometimes adding a new directory in third_party without a LICENSE file
can cause the script to fail ([example](https://bugs.chromium.org/p/skia/issues/detail?id=8027)).

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
  * ../../../third_party/dart/tools/sdks/dart-sdk/bin/pub get
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
* The code for the pre-upload license step used by the autoroller is [here](https://skia.googlesource.com/buildbot/+/master/autoroll/go/repo_manager/pre_upload_steps.go).
