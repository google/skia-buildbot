tracedb
=======

The traceserver and tools for talking to traceservers.

traceserver
-----------

The traceserver is a simple gRPC server that serves up the traceservice
endpoint. In production we run the following services at these ports:

  Server        |  Port  |  Data
  --------------|--------|--------
  skia-tracedb  |  9000  |  Perf
  skia-tracedb  | 10000  |  Gold

These ports are not available externally from GCE so to access them from your
desktop there are two helper scripts, `perf_tunnel.sh` and `gold_tunnel.sh`
which will set up SSH port forwarding from localhost:9090 to skia-tracedb:9000
and skia-tracedb:10000 respectively.

Note that `tracetool` defaults to trying to talk to the endpoint on
localhost:9090, so you shouldn't need to pass the --address argument
to `tracetool` when using the port forwaring scripts.
