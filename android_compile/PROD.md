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
or from the [cloud logs](goto/skia-android-framework-compile-bot-cloud-logs).
Pending tasks can also be deleted if absolutely necessary
[here](goto/skia-android-framework-compile-bot-datastore).


android_tree_broken
-------------------

The Android Compile Bot thinks that the android tree is broken and is allowing
Skia CLs to pass because the withpatch and nopatch builds are both green.
Look at the android dashboard [here](goto/ab). Also look at task logs in
the datastore [here](goto/skia-android-framework-compile-bot-datastore).


infra_failure
-------------

Atleast one compile task failed due to an infra failure. Look at task logs in
the datastore [here](goto/skia-android-framework-compile-bot-datastore) and
look for errors in the
[cloud logs](goto/skia-android-framework-compile-bot-cloud-logs).
