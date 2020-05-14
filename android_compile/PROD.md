Android Compile Server Production Manual
========================================

General information about the Android compile server is available in the
[README](./README.md).


Alerts
======


queue_too_long
--------------

The number of waiting compile tasks on Android Compile Server is too long.
Take a look at the pending tasks [here](https://chromium-swarm.appspot.com/tasklist?c=name&c=state&c=created_ts&c=duration&c=pending_time&c=pool&c=bot&c=sk_issue&c=sk_patchset&f=sk_issue_server%3Ahttps%3A%2F%2Fskia-review.googlesource.com&f=sk_name-tag%3ABuild-Debian9-Clang-cf_x86_phone-eng-Android_Framework&l=50&n=true&s=created_ts%3Adesc) and [here](https://chromium-swarm.appspot.com/tasklist?c=name&c=state&c=created_ts&c=duration&c=pending_time&c=pool&c=bot&c=sk_issue&c=sk_patchset&et=1517930880000&f=sk_issue_server%3Ahttps%3A%2F%2Fskia-review.googlesource.com&f=sk_name-tag%3ABuild-Debian9-Clang-host-sdk-Android_Framework&l=50&n=true&s=created_ts%3Adesc&st=1517585280000).
Try to determine if the backends listed [here](https://console.cloud.google.com/datastore/entities;kind=AndroidCompileInstances;ns=android-compile/query/kind?project=google.com:skia-corp) are still running in skia-corp.
Pending tasks can also be deleted if absolutely necessary
[here](https://goto.google.com/skia-android-framework-compile-bot-datastore).


mirror_sync_failed
------------------

The mirror sync failed. This will likely cause all checkouts of the affected backend
to also fail when syncing from the mirror.

Try syncing the mirrors again [here](https://skia-android-compile.corp.goog/).

If things still fail then try to log into the backend instance on skia-corp and run:
* cd /mnt/pd0/checkouts/mirror/
* repo init -u https://googleplex-android.googlesource.com/a/platform/manifest -b master --mirror
* repo sync -j100 --optimized-fetch --prune


android_tree_broken
-------------------

At least one backend of the Android Compile Bot thinks that the android tree is
broken and is allowing Skia CLs to pass because the withpatch and nopatch builds
are both red. Verify that the tree is really broken by looking at the android dashboard
[here](https://goto.google.com/ab).
If it is not broken then look at task logs in
the datastore [here](https://goto.google.com/skia-android-framework-compile-bot-datastore)
to see if it is only affecting one backend (look for NoPatchLog to exist and
NoPatchSucceeded to be false). If it is only affecting one backend then try syncing
the mirrors again [here](https://skia-android-compile.corp.goog/).

If nothing else works then make the bot an experimental bot in [commit-queue.cfg](https://skia.googlesource.com/skia/+show/infra/config/commit-queue.cfg)
and inform the Skia chat so developers are cautious with potentially breaking changes.
Also contact the [robocop](http://tree-status.skia.org/robocop) with the
contents of the failing NoPatchLog.


android_checkout_sync_failure
-----------------------------

At least one checkout is failing to sync. Look at task logs in
the datastore [here](https://goto.google.com/skia-android-framework-compile-bot-datastore)
to see if it is only affecting one backend (look for NoPatchLog to exist and
NoPatchSucceeded to be false). If it is only affecting one backend then try syncing
the mirrors again [here](https://skia-android-compile.corp.goog/).

If nothing else works then make the bot an experimental bot in [commit-queue.cfg](https://skia.googlesource.com/skia/+show/infra/config/commit-queue.cfg)
and inform the Skia chat so developers are cautious with potentially breaking changes.
Also contact the [robocop](http://tree-status.skia.org/robocop) with the
contents of the failing NoPatchLog.
