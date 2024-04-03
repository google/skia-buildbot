This directory contains the backend service implementation and it's respective controllers.

The backend service is intended to host api endpoints that are not directly user facing.
This helps isolate user facing components from the other services running in the cluster by
providing a standard interface contract that calling services can adhere to. For example, to run
manual pinpoint jobs from the UI, the frontend service can invoke the endpoint on the backend
service. The backend endpoint will trigger the workflow in the temporal cluster. In the future if
we replace temporal with another solution, the contract between frontend (and any other service
that invokes this endpoint) remains unchanged and makes the code easy to upgrade. This service can
also be used to lighten the load on the frontend service by offloading some heavier operations
(eg: Dry run regression detection) that are currently being run in frontend to the backend service.

# Running the backend service locally

To run a local instance of the backend service, simply run the cmd below from the [perf](../../)
directory.

`make run-demo-backend`

# Adding a new endpoint

The endpoint controllers are all in the `controllers` directory. To add a new endpoint, please
follow the steps below. A sample `hello` endpoint is provided for reference.

1. Create a new proto definition defining your service. This need not be under this directory and
   can reside in a location specific to your implementation.
   Eg: [culprit_service.proto](../culprit/proto/v1/culprit_service.proto).
2. Create a generate.go file that provides the generation script to create the stubbed
   client/server code for your service. Eg:
   [generate.go](../culprit/proto/v1/generate.go). Replace the arguments on the
   `//:protoc` command to specify your module and input proto file appropriately.
   Running `go generate ./...` in the location of this file will generate the stubs.
3. Create your endpoint controller by implementing implement the `backend.BackendService` interface.
   Eg: [culprit service](../culprit/service/service.go)
4. Add the newly added service to the list in `initialize()` function in [backend.go](backend.go).

The endpoints are simple grpc apis, so you can either create a client for the specific endpoint
using the stubs, or use a tool like [grpc_cli](http://go/grpc_cli).

# Checking available endpoints

`grpc_cli ls [::]:8005 --channel_creds_type=local`

# Invoking an endpoint locally

`grpc_cli call [::]:8005 --channel_creds_type=local backend.v1.Hello.SayHello "text: 'World'"`

# Creating Go clients for your service

In order to make the grpc connections uniform across the services, we have created a
[backendclientutility](client/backendclientutil.go) to abstract out the GRPC boilerplate
and make it easy to create clients for individual services. If you are adding a new service,
simply follow the pattern in this file to create a function to create the client for your service.

# URL for the Backend Service

For local deployments, you can simply use `localhost:8005` (double check the value specified
in [demo.json](../../configs/demo.json)).
For production or GKE deployment of the Backend service, we use Kubernetes DNS. Since the endpoints
on this service are intended to be internal S2S calls only, we are not opening up external DNS
to the service. Instead we can use the service cluster url
(eg: `perf-be-chrome-non-public.perf.svc.cluster.local:7000`) to invoke the BE kubernetes workload.
The service name and port will be different for each instance, so please refer to the table below
to get the url for the instance you want.

| Instance          | URL                                                   |
| ----------------- | ----------------------------------------------------- |
| Chrome (Internal) | perf-be-chrome-non-public.perf.svc.cluster.local:7000 |
