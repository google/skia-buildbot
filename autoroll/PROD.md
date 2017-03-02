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

