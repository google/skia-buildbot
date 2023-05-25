# CABE

CABE is a performance benchmark A/B experiment analysis service.

See the [Design Doc](http://go/cabe-rpc).

## Code structure, bazel targets of interest

- `//cabe:cabeserver`
  - main app container for deploying the cabe service to GKE
- `//cabe/go/analysisserver`
  - library that implements cabe's gRPC Analysis service proto interface
- `//cabe/go/cmd/...`
  - location of subpackages for executable entry points (ie `go_binary` targets)
- `//cabe/go/cmd/cabeserver`
  - main entry point for the cabe server binary
  - handles CLI flags, non-functional details like auth settings etc
  - depends on `//cabe/go/analysisserver` for the actual request handler implementations
- `//cabe/go/proto`
  - protobuf message and service definitions

## Querying the production cabe.skia.org service with `grpcurl`

To make a `GetAnalysis` request for a specific pinpoint job, you can use the
[grpcurl](https://github.com/fullstorydev/grpcurl) command line utility.

For this command to work, you should also have `gcloud` installed, and run
`gcloud auth login` so you're logged in using an authorized account.

```
grpcurl -H "Authorization: Bearer $(gcloud auth print-access-token)" \
    -d '{"pinpoint_job_id":"<PINPOINT JOB ID>"}' \
    cabe.skia.org:443 cabe.proto.Analysis/GetAnalysis
```

If successful, this command will write a textproto encoding of the
`cabe.proto.GetAnalysisResponse` from the server to stdout.

## Running locally

To start the server, in one terminal run:

```
bazelisk run //cabe/go/cmd/cabeserver -- -disable_grpcsp
```

This should start the gRPC service and print out some log messages
including the port that server is listening on. Currently, the
default is `50051` though you can specify it (and other flags) like
so:

```
bazelisk run //cabe/go/cmd/cabeserver -- -disable_grpcsp -grpc_port <some other port>
```

Once the server process has started, you should be able to use
[grpcurl](https://github.com/fullstorydev/grpcurl) to make calls to it
on your workstation. For example:

```
grpcurl -vv -plaintext 127.0.0.1:50051 cabe.proto.Analysis/GetAnalysis
```

## Running locally with local auth-proxy

Hopefully you will never need to do this, but just in case you need
to debug problems with authentication or authorization here's how
to run skia's `auth-proxy` in front of `cabe`, both running on your
workstation.

In one terminal, run auth-proxy:

```
bazelisk run //kube/cmd/auth-proxy -- \
    --prom-port=:20001 \
    --role=viewer=google.com \
    --authtype=mocked \
    --mock_user=$USER@google.com \
    --port=:8003 \
    --target_port=https://127.0.0.1:50051 \
    --self_sign_localhost_tls \
    --local
```

In another terminal, start cabserver (no not use `-disable_grpcsp` with this method):

```
bazelisk run //cabe/go/cmd/cabeserver
```

Then in a third terminal, use grpcurl to send a request through the auth-proxy
to the cabeserver (make sure you have the `-insecure` flag set):

```
grpcurl -vv -insecure 127.0.0.1:8003 cabe.proto.Analysis/GetAnalysis
```

Note that this command will produce a warning message (which you can ignore for
local debugging purposes) about disabling SSL verification.

`cabserver` should be able to see the auth headers set by `auth-proxy` now, and the
user identity should be the `$USER@google.com` specified in the `mock_user` flag
passed to the `auth-proxy` command.
