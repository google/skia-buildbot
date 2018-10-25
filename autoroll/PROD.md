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

The Skia->Flutter roller may throw errors from Flutter's license script:
"Failed to transition from "idle" to "active": Error when running pre-upload step: Error when running dart license script: Command exited with exit status 1: /data/engine/src/third_party/dart/tools/sdks/dart-sdk/bin/dart lib/main.dart --release --src ../../.. --out /data/engine/src/out/licenses"
This alert means that the licence script is failing possibly due to a recent
Skia change. Ask brianosman@ to run the script manually, or follow these steps
on the roller to see what failed:
* cd /data/engine/src/flutter/
* Check to see if DEPS has the latest Skia rev. If not update it and run "/data/depot_tools/gclient sync"
* cd /data/engine/src/flutter/tools/licenses/
* /data/engine/src/third_party/dart/tools/sdks/dart-sdk/bin/dart lib/main.dart --release --src ../../.. --out /data/engine/src/out/licenses
