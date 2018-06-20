Android Compile Server Production Manual
========================================

General information about the Android compile server is available in the
[README](./README.md).


Alerts
======


queue_too_long
--------------

The number of waiting compile tasks on Android Compile Server is too long.
Take a look at the pending tasks [here](https://android-compile.skia.org/).
Try to determine if current running tasks are taking too long from the
[graphs](https://mon.skia.org/dashboard/db/android-compile-bot)
or from the [cloud logs](https://goto.google.com/skia-android-framework-compile-bot-cloud-logs).
Pending tasks can also be deleted if absolutely necessary
[here](https://goto.google.com/skia-android-framework-compile-bot-datastore).


android_tree_broken
-------------------

The Android Compile Bot thinks that the android tree is broken and is allowing
Skia CLs to pass because the withpatch and nopatch builds are both red.
Look at the android dashboard [here](https://goto.google.com/ab). Also look at task logs in
the datastore [here](https://goto.google.com/skia-android-framework-compile-bot-datastore).


infra_failure
-------------

Atleast one compile task failed due to an infra failure. Look for errors in the
[cloud logs](https://goto.google.com/skia-android-framework-compile-bot-cloud-logs-errors).

If the error appears to be an Android infrastructure issue (eg: sync problems because
repository is down) and it does not resolve soon, then make the bot an experimental bot
in [cq.cfg](https://skia.googlesource.com/skia/+/master/infra/branch-config/cq.cfg).
Add it back to the regular CQ after the infrastructure problem eventually resolves.
