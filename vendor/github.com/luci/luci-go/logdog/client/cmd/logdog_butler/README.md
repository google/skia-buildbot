LogDog Butler
=============

The LogDog Butler runs on log-producing clients and manages the ingestion of
log data from a read-forward stream, formatting and bundling it into [log
protobufs](../../../api/logpb), and the transmission of those protobufs through
the **Transport Layer** to **Collector** instances.

The Butler is fairly configurable, and is capable of being used in multiple
scenarios via its subcommands:

* `stream`, which uploads the contents of a single log stream.
* `serve`, which runs a Butler stream server instance.
* `run`, which runs another command, instrumenting its enviornment and standard
  file descriptors for log streaming.

## Stream Servers

The Butler, based on its configuration, may configure default streams. For
example, when using the `run` subcommand, it has the option to connect the
bootstrapped process' `STDOUT` and `STDERR` streams to LogDog streams.

However, the Butler can also enable additional stream registration by
instantiating a local **Stream Server** and exposing that to other applications.
Those applications can then connect to the **Stream Server**, perform a minimial
[handshake](../../butlerproto), and connect an additional stream to the Butler.

Clients can use the [streamclient](../../butlerlib/streamclient) package to
connect to a stream server and create Butler streams. Additional stream client
packages are availalbe for other languages:

* [Python](https://github.com/luci/luci-py/tree/master/client/libs/logdog)

**Stream Servers** are created by specifying the `-streamserver-uri` parameter
to LogDog. Stream server options vary by operating system.

### POSIX

POSIX systems (Linux, Mac) support the following stream servers:

* `unix:<path>`, where `path` is a filesystem path to a named UNIX domain socket
  to create.

### Windows

Windows systems support the following stream servers:

* `net.pipe:<name>`, where `name` is a valid Windows named pipe name.

## Production

In production, each Butler instance will begin by registering a unique log
stream Prefix with its **Coordinator** instance. The exchange will ensure that
only that specific Butler instance may create log streams underneath of that
Prefix.

Production streaming can be requested by specifying the `logdog` Output option
and associated parameters:

```shell
$ logdog_butler -output logdog,host=<coordinator-host> ...
```

The Butler instance must be configured to use a service account that has
**WRITE** permission for the target project.

This will cause the Butler to perform prefix registration during its Output
initialization, prior to any bootstrapping or streaming.
