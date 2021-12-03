Task Drivers
============

Task Drivers are Go programs that facilitate the compilation of code, the execution of tests, and
other actions as a part of our Continuous Integration system.

They run a series of steps and store logs in such a way that they can be viewed in a logical manner.

Task Driver Server
------------------
The purpose of ./go/task-driver-server is to ingest logs from the execution of task drivers and
to present those collected logs.

When running, a task driver will stream logs and step metadata to both stdout to
[Stack Driver Logging](https://cloud.google.com/logging). There is a
[sink](https://cloud.google.com/logging/docs/export/configure_export_v2) set up that will forward
these logs to [Pub/Sub](https://cloud.google.com/pubsub/docs/overview) [1]. This server subscribes
to the Pub/Sub topic [2] and processes the metadata by storing it to BigTable.

See [the design doc for more details](https://docs.google.com/document/d/1BqbHKD2TWthA0XhidCxriqIWzutRGgc0qxbfRgSDijs/edit)

 1. The sink is called `task-driver-logs-to-pubsub`.
[cloud console](https://console.cloud.google.com/logs/router?project=skia-swarming-bots)
 2. The topic is called `projects/skia-swarming-bots/topics/task-driver-logs`
 [cloud console](https://console.cloud.google.com/cloudpubsub/topic/detail/task-driver-logs?project=skia-swarming-bots)