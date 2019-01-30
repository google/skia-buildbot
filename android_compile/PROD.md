Android Compile Server Production Manual
========================================

General information about the Android compile server is available in the
[README](./README.md).


Alerts
======


queue_too_long
--------------

The number of waiting compile tasks on Android Compile Server is too long.
Take a look at the pending tasks [here](https://chromium-swarm.appspot.com/tasklist?c=name&c=state&c=created_ts&c=duration&c=pending_time&c=pool&c=bot&c=sk_issue&c=sk_patchset&f=sk_issue_server%3Ahttps%3A%2F%2Fskia-review.googlesource.com&f=sk_name-tag%3ABuild-Debian9-Clang-cf_x86_phone-eng-Android_Framework&l=50&n=true&s=created_ts%3Adesc).
Try to determine if current running tasks are taking too long by port forwarding
to skia-corp with `kubectl port-forward prometheus-0 9090 9090` and bring up graphs of [sync times](http://localhost:9090/graph?g0.range_input=1d&g0.expr=android_sync_time_checkout_1%7Bapp%3D%22android-compile%22%7D%2F60&g0.tab=0&g1.range_input=1d&g1.expr=android_sync_time_checkout_2%7Bapp%3D%22android-compile%22%7D%2F60&g1.tab=0&g2.range_input=1d&g2.expr=android_sync_time_checkout_3%7Bapp%3D%22android-compile%22%7D%2F60&g2.tab=0&g3.range_input=1d&g3.expr=android_sync_time_checkout_4%7Bapp%3D%22android-compile%22%7D%2F60&g3.tab=0&g4.range_input=1d&g4.expr=android_sync_time_checkout_5%7Bapp%3D%22android-compile%22%7D%2F60&g4.tab=0&g5.range_input=1d&g5.expr=android_sync_time_mirror%7Bapp%3D%22android-compile%22%7D%2F60&g5.tab=0) and [compile times](http://localhost:9090/graph?g0.range_input=1d&g0.expr=android_compile_time_checkout_1%7Bapp%3D%22android-compile%22%7D%2F60&g0.tab=0&g1.range_input=1d&g1.expr=android_compile_time_checkout_2%7Bapp%3D%22android-compile%22%7D%2F60&g1.tab=0&g2.range_input=1d&g2.expr=android_compile_time_checkout_3%7Bapp%3D%22android-compile%22%7D%2F60&g2.tab=0&g3.range_input=1d&g3.expr=android_compile_time_checkout_4%7Bapp%3D%22android-compile%22%7D%2F60&g3.tab=0&g4.range_input=1d&g4.expr=android_compile_time_checkout_5%7Bapp%3D%22android-compile%22%7D%2F60&g4.tab=0).
Pending tasks can also be deleted if absolutely necessary
[here](https://goto.google.com/skia-android-framework-compile-bot-datastore).


mirror_sync_failed
------------------

The mirror sync failed. This will likely cause all checkouts to also fail when
syncing from the mirror. Fix this by logging into android-compile on skia-corp
and running:
* cd /mnt/pd0/checkouts/mirror/
* repo init -u https://googleplex-android.googlesource.com/a/platform/manifest -g "all,-notdefault,-darwin" -b master --mirror
* repo sync -c -j100 --optimized-fetch --prune -f


android_tree_broken
-------------------

The Android Compile Bot thinks that the android tree is broken and is allowing
Skia CLs to pass because the withpatch and nopatch builds are both red.
Verify that the tree is really broken by looking at the android dashboard
[here](https://goto.google.com/ab). Also look at task logs in
the datastore [here](https://goto.google.com/skia-android-framework-compile-bot-datastore).


infra_failure
-------------

Atleast one compile task failed due to an infra failure. Look for errors in the
[cloud logs](https://goto.google.com/skia-android-framework-compile-bot-cloud-logs-errors).

If the error appears to be an Android infrastructure issue (eg: sync problems because
repository is down) and it does not resolve soon, then make the bot an experimental bot
in [commit-queue.cfg](https://skia.googlesource.com/skia/+/infra/config/commit-queue.cfg).
Add it back to the regular CQ after the infrastructure problem eventually resolves.
