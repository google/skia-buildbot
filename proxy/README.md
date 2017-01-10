skia-proxy
==========

A proxy that only allows auth'd connections, and allows HTTP access to any
server and any port in the GCE project.

A single server that handles all requests to hosts of the form:

    server-name-10000-proxy.skia.org

Where 'server-name' is the internal name of the GCE instance, and '10000' is
the port number to connect to.
