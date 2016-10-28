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

The specified trybot in the specified CL has been running beyond the threshold.
Try to determine why it took so long: Was it in pending state? is it a one-off
event? is this the new norm and the threshold needs to be increased?

too_many_trybots_triggered
--------------------------

The number of triggered CQ trybots by the specified CL is beyond the threshold.
This alert has been created to detect problems like
[crbug/656756](https://bugs.chromium.org/p/chromium/issues/detail?id=656756).

