LogDog
======

LogDog is a high-performance log collection and dissemination platform. It is
designed to collect log data from a large number of cooperative individual
sources and make it available to users and systems for consumption. It is
composed of several services and tools that work cooperatively to provide a
reliable log streaming platform.

Like other LUCI components, LogDog primarily aims to be useful to the
[Chromium](https://www.chromium.org/) project.

LogDog offers several useful features:

* Log data is streamed, and is consequently available the moment that it is
  ingested in the system.
* Flexible hierarchial log namespaces for organization and navigation.
* Recognition of different projects, and application of different ACLs for each
  project.
* Able to stream text, binary data, or records.
* Long term (possibly indefinite) log data storage and access.
* Log data is sourced from read-forward streams (think files, sockets, etc.).
* Leverages the LUCI Configuration Service for configuration and management.
* Log data is implemented as [protobufs](api/logpb/log.proto).
* The entire platform is written in Go.
* Rich metadata is collected and stored alongside log records.
* Built entirely on scalable platform technologies, targeting Google Cloud
  Platform.
  * Resource requirements scale linearly with log volume.


## APIs

Most applications will interact with a LogDog Coordinator instance via its
[Coordinator Logs API](api/endpoints/coordinator/logs/v1).


## Life of a Log Stream

Log streams pass through several layers and states during their path from
generation through archival.

1. **Streaming**: A log stream is being emitted by a **Butler** instance and
   pushed through the **Transport Layer** to the **Collector**.
  1. **Pre-Registration**: The log stream hasn't been observed by a
     **Collector** instance yet, and exists only in the mind of the **Butler**
     and the **Transport** layer.
  1. **Registered**: The log stream has been observed by a **Collector**
     instance and successfully registered with the **Coordinator**. At this
     point, it becomes queryable, listable, and the records that have been
     loaded into **Intermediate Storage** are streamable.
1. **ArchivePending**: One of the following events cause the log stream to be
   recognized as finished and have an archival request dispatched. The archival
   request is submitted to the **Archivist** cluster.
   * The log stream's terminal entry is collected, and the terminal index is
     successfully registered with the **Coordinator**.
   * A sufficient amount of time has expired since the log stream's
     registration.
1. **Archived**: An **Archivist** instance has received an archival request for
   the log stream, successfully executed the request according to its
   parameters, and updated the log stream's state with the **Coordinator**.


Most of the lifecycle is hidden from the Logs API endpoint by design. The user
need not distinguish between a stream that is streaming, has archival pending,
or has been archived. They will issue the same `Get` requests and receive the
same log stream data.

A user may differentiate between a streaming and a complete log by observing its
terminal index, which will be `< 0` if the log stream is still streaming.


## Components

The LogDog platform consists of several components:

* [Coordinator](appengine/coordinator), a hosted service which serves log data
  to users and manages the log stream lifecycle.
* [Butler](client/cmd/logdog_butler), which runs on each log stream producing
  system and serves log data to the Collector for consumption.
* [Collector](server/cmd/logdog_collector), a microservice which takes log
  stream data and ingests it into intermediate storage for streaming and
  archival.
* [Archivist](server/cmd/logdog_archivist), a microservice which compacts
  completed log streams and prepares them for long-term storage.

LogDog offers several log stream clients to query and consume log data:

* [LogDog Cat](client/cmd/logdog_cat), a CLI to query and view log streams.
* [Web App](/web/apps/logdog-app), a heavy log stream navigation
  application built in [Polymer](https://www.polymer-project.org).
* [Web Viewer](/web/apps/logdog-view), a lightweight log stream viewer built in
  [Polymer](https://www.polymer-project.org).

Additionally, LogDog is built on several abstract middleware technologies,
including:

* A **Transport**, a layer for the **Butler** to send data to the **Collector**.
* An **Intermediate Storage**, a fast highly-accessible layer which stores log
  data immediately ingested by the **Collector** until it can be archived.
* An **Archival Storage**, for cheap long-term file storage.

Log data is sent from the **Butler** through **Transport** to the **Collector**,
which stages it in **Intermediate Storage**. Once the log stream is complete
(or expired), the **Archivist** moves the data from **Intermediate Storage** to
**Archival Storage**, where it will permanently reside.

The Chromium-deployed LogDog service uses
[Google Cloud Platform](https://cloud.google.com/) for several of the middleware
layers:

* [Google AppEngine](https://cloud.google.com/appengine), a scaling application
  hosting service.
* [Cloud Datastore](https://cloud.google.com/datastore/), a powerful
  transactional NOSQL structured data storage system. This is used by the
  Coordinator to store log stream state.
* [Cloud Pub/Sub](https://cloud.google.com/pubsub/), a publish / subscribe model
  transport layer. This is used to ferry log data from **Butler** instances to
  **Collector** instances for ingest.
* [Cloud BigTable](https://cloud.google.com/bigtable/), an unstructured
  key/value storage. This is used as **intermediate storage** for log stream
  data.
* [Cloud Storage](https://cloud.google.com/storage/), used for long-term log
  stream archival storage.
* [Container Engine](https://cloud.google.com/container-engine/), which manages
  Kubernetes clusters. This is used to host the **Collector** and **Archivist**
  microservices.

Additionally, other LUCI services are used, including:

* [Auth Service](https://github.com/luci/luci-py/tree/master/appengine/auth_service),
  a configurable hosted access control system.
* [Configuration Service](https://github.com/luci/luci-py/tree/master/appengine/config_service),
  a simple repository-based configuration service.

## Instantiation

To instantiate your own LogDog instance, you will need the following
prerequisites:

* A **Configuration Service** instance.
* A Google Cloud Platform project configured with:
  * Datastore
  * A Pub/Sub topic (Butler) and subscription (Collector) for log streaming.
  * A Pub/Sub topic (Coordinator) and subscription (Archivist) for archival
    coordination.
  * A Container Engine instance for microservice hosting.
  * A BigTable cluster.
  * A Cloud Storage bucket for archival staging and storage.

Other compatible optional components include:

* An **Auth Service** instance to manage authentication. This is necessary if
  something stricter than public read/write is desired.

### Config

The **Configuration Service** must have a valid service entry text protobuf for
this LogDog service (defined in
[svcconfig/config.proto](api/config/svcconfig/config.proto)).

### Coordinator

After deploying the Coordiantor to a suitable cloud project, several
configuration parameters must be defined visit its settings page at:
`https://<your-app>/admin/settings`, and configure:

* Configure the "Configuration Service Settings" to point to the **Configuration
  Service** instance.
* Update "Tumble Settings" appropriate (see [tumble docs](/tumble)).
* If using timeseries monitoring, update the "Time Series Monitoring Settings".
* If using **Auth Service**, set the "Authorization Settings".

If you are using a BigTable instance outside of your cloud project (e.g.,
staging, dev), you will need to add your BigTable service account JSON to the
service's settings. Currently this cannot be done without a command-line tool.
Hopefully a proper settings page will be added to enable this, or alternatiely
Cloud BigTable will be updated to support IAM.

### Microservices

Microservices are hosted in Google Container Engine, and use Google Compute
Engine metadata for configuration.

The following metadata parameters **must** be set for deployed microservices
to work:

* `logdog_coordinator_host`, the host name of the Coordinator service.

All deployed microservices use the following optional metadata parameters for
configuration:

* `logdog_storage_auth_json`, an optional file containing the authentication
  credentials for intermediate storage (i.e., BigTable). This isn't necessary
  if the BigTable node is hosted in the same cloud project as the microservice
  is running, and the microservice's container has BigTable Read/Write
  permissions.
* `tsmon_endpoint`, an optional endpoint for timeseries monitoring data.

#### Collector

The Collector instance is fully command-line compatible. Its [entry point
script](server/cmd/logdog_collector/run.sh) uses Google Compute Engine metadata
to populate the command line in a production enviornment:

* `logdog_collector_log_level`, an optional `-log-level` flag value.

#### Archivist

The Archivist instance is fully command-line compatible. Its [entry point
script](server/cmd/logdog_archivist/run.sh) uses Google Compute Engine metadata
to populate the command line in a production enviornment:

* `logdog_archivist_log_level`, an optional `-log-level` flag value.
