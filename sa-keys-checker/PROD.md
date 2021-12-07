Service Account Keys Checker Production Manual
==============================================

The goal of this service is to add metrics for when the keys of the service
accounts in Skia's cloud projects are going to expire, so we can get alerts
based on them.

Alerts
======

Items below here should include target links from alerts.

sa_key_expiring_soon
--------------------

This alert signifies that the specified service account's key is expiring
within 30 days.

Create a new key to replace it or directly delete it if it is no longer
required.

You can use [refresh_jumphost-service-account.sh](https://skia.googlesource.com/buildbot/+/main/skolo/refresh-jumphost-service-account.sh)
for Skolo jumphost service accounts.

For k8s services, the steps in [this comment](https://bugs.chromium.org/p/skia/issues/detail?id=12496#c1)
will be useful.

Key metrics: sa_key_expiration_s

sa_key_expired
--------------

This alert signifies that the specified service account's key has expired.

Delete the expired key from pantheon if it is no longer required.

Key metrics: sa_key_expiration_s
