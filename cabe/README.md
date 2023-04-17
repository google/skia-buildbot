# CABE

CABE is a performance benchmark A/B experiment analysis service.

See the [Design Doc](http://go/cabe-rpc).

## Code structure

    go/cabeserver
        cabe rpc server process main package
    go/proto
        protobuf message and service definitions

## Running locally

```
bazelisk run //cabe/go/cabeserver
```

## Running locally with local auth-proxy

In one terminal, run auth-proxy:
```
bazelisk run kube/cmd/auth-proxy -- \
    --prom-port=:20001 \
    --role=viewer=google.com \
    --authtype=mocked \
    --mock_user=$USER@google.com \
    --port=:8003 \
    --target_port=https://127.0.0.1:50051 \
    --self_sign_localhost_tls \
    --local
```

In another terminal, start cabserver:
```
bazelisk run //cabe/go/cabeserver
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