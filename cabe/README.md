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