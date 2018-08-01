CQ Watcher Production Manual
============================

General information about the CQ Watcher is available in the
[README](./README.md).


Alerts
======

too_many_cls
------------

The number of CLs in Skia's CQ are beyond the threshold. Take a look at the
[dry run queue](https://skia-review.googlesource.com/q/label:Commit-Queue%253D1+status:open)
and the [commit queue](https://skia-review.googlesource.com/q/label:Commit-Queue%253D2+status:open)
and try to determine why CLs have piled up there.

trybot_duration_beyond_threshold
--------------------------------

The specified trybot has been running beyond the threshold. Look for which CLs
this happened in from the [cloud logs](https://pantheon.corp.google.com/logs/viewer?project=skia-public&advancedFilter=logName%3D%22projects%2Fskia-public%2Flogs%2Fcq-watcher%22%20AND%20textPayload:%20%22CQTrybotDurationError%22).
Try to determine why it took so long: Was it in pending state? is it a one-off
event? is this the new norm and the threshold needs to be increased?

too_many_trybots_triggered
--------------------------

The number of triggered CQ trybots by atleast one CL is beyond the threshold.
Look for which CLs this happened in from the [cloud logs](https://pantheon.corp.google.com/logs/viewer?project=skia-public&advancedFilter=logName%3D%22projects%2Fskia-public%2Flogs%2Fcq-watcher%22%20AND%20textPayload:%20%22CQCLsCountError%22).
This alert has been created to detect problems like
[crbug/656756](https://bugs.chromium.org/p/chromium/issues/detail?id=656756).

