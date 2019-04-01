alert-manager Production Manual
===============================

First make sure you are familiar with the design of alert-manager by reading the
[DESIGN](./DESIGN.md) doc.

Alerts
======

Items below here should include target links from alerts.

alert_to_pubsub
---------------

Every 15s the alert-to-pubsub instance should receive a POST request from the
local Prometheus instance and then convert all of those alerts into PubSub
events, including a `healthz` PubSub event. This alert is fired if a location,
like `skia-corp` has failed to generate a `healthz` event recently. Check
that both Prometheus and alert-to-pubsub are running in the designated
cluster. Also check the PubSub Topic and Subscriptions.
