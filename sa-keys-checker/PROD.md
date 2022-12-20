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

If running this script fails with:

        ERROR: (gcloud.beta.iam.service-accounts.keys.create) FAILED_PRECONDITION: Precondition check failed.

Then that means the service account has too many keys (10 is the limit)
and you will need to delete old expired keys before creating a new key.

To confirm that all the metadata servers have restarted you can run:

        ansible  jumphosts -a "ps aux" | grep metadata

For k8s services in skia-corp, you can use the [rotate-keys-for-skia-corp-sa.sh](https://skia.googlesource.com/buildbot/+/main/kube/secrets/rotate-keys-for-skia-corp-sa.sh) script. Example:

        bash secrets/rotate-keys-for-skia-corp-sa.sh google.com:skia-corp alert-to-pubsub deployment/alert-to-pubsub

Key metrics: sa_key_expiration_s

sa_key_expired
--------------

This alert signifies that the specified service account's key has expired.

Delete the expired key from pantheon if it is no longer required.

Key metrics: sa_key_expiration_s

