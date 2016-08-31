LogDog Coordinator
==================

The LogDog Coordinator is an AppEngine application that serves as a central
coordinating and interactive authority for a LogDog instance. The Coordinator
is responsible for:

* Supplying logs to end users through its logs API.
* Coordinating log stream state throughout its lifecycle.
* Handling Butler Prefix registration.
* Acting as a configuration authority for its deployment.
* Accepting stream registrations from **Collector** instances.
* Dispatching archival tasks to **Archivist** instances.

## Modules

A **Coordinator** occupies the AppEngine space of a given cloud project, and
assumes ownership of that project's resources. It is composed of several
cooperative AppEngine modules.

### Default

The [default](vmuser/) module exposes the
[Logs API](../../../api/endpoints/coordinator/logs/v1/) for log stream querying
and consumption.

It is currently an AppEngine Managed VM, since Managed VMs are the only type of
AppEngine instance that can use gRPC, and gRPC is needed to read from BigTable.

### Services

The [services](services/) module exposes management endpoints to the instance's
microservices, notably the [Collector](../../../server/cmd/logdog_collector) and
[Archivist](../../../server/cmd/logdog_archivist) microservices. These endpoints
are used to coordinate the microservice-managed aspects of the log stream
lifecycle.

### Backend

The [backend](backend/) module hosts the [Tumble](/tumble) journaled processing
service, which handles log stream lifecycle transitions and deadlines.

### Static

The [static](static/) module hosts static content, including:
* The LogDog Web Application
* The LogDog Lightweight Stream Viewer
* `rpcexplorer`
