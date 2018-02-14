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
Try to determine if current running tasks are taking too long from the graphs
or from the [cloud logs](goto/skia-android-framework-compile-bot-cloud-logs).
Pending tasks can also be deleted if absolutely necessary
[here](goto/skia-android-framework-compile-bot-datastore).
