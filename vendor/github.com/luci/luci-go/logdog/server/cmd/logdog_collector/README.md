LogDog Collector
================

The LogDog **Collector** is configured to receive
[Butler](../../../client/cmd/logdog_butler)-generated logs from the **Transport
Layer**, register them with the **Coordinator** instance, and load them into
**Intermediate Storage** for streaming and, ultimately, archival.

**Collector** microservices are designed to operate cooperatively as part of
a scaleable cluster. Deploying additional **Collector** instances will linearly
increase the collection throughput.
