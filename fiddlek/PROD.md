Fiddle Production Manual
========================

First make sure you are familiar with the design of fiddle by reading the
[DESIGN](./DESIGN.md) doc.

Alerts
======

Items below here should include target links from alerts.

fiddler_pods
------------

Fiddle should run enough fiddler pods that the number of idle fiddle pods never
gets too low. Check the amount of traffic that fiddle is receiving and if the
traffic is legitimate then increase the number of replicas in the fiddler.yaml
file.
