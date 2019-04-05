Skia Status Production Manual
=============================

General information about Skia Status is available in the [README](./README.md).


Troubleshooting
===============

incremental_cache_failed
------------------------

The incrementally-updating cache has failed to update for too long. This is
usually due to an outage of an upstream service, like Firestore or Git. Check
the logs and if this is the case, file bugs or ping teams to ensure that they
are aware. If the problem is fixable on our end, file a bug or fix it directly.


http_latency
------------

The frontend is taking too long to respond. Check to see whether the server is
overloaded (does the pod need more cores or RAM?), and check the logs for
errors.
