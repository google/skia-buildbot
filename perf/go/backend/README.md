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
   Eg: [placeholder/proto/service.proto](placeholder/proto/service.proto).
2. Create a generate.go file that provides the generation script to create the stubbed
   client/server code for your service. Eg:
   [placeholder/proto/generate.go](placeholder/proto/generate.go). Replace the arguments on the
   `//:protoc` command to specify your module and input proto file appropriately.
   Running `go generate ./...` in the location of this file will generate the stubs.
3. Add your endpoint controller under [services](services) directory as a separate go pkg. The
   service you add needs to implement the `backend.BackendService` interface.
4. Add the newly added service to the list in `initialize()` function in [backend.go](backend.go).

# Invoking an endpoint locally

The endpoints are simple grpc apis, so you can either create a client for the specific endpoint
using the stubs, or use a tool like [grpc_cli](go/grpc_cli). For eg: the SayHello placeholder
endpoint can be invoked by

`grpc_cli call [::]:8005 --channel_creds_type=local backend.v1.Hello.SayHello "text: 'World'"`
