RPC Module
==========

This module provides interaction points with the server.  It contains two
generated files, rpc.ts and twirp.ts, which contain the RPC types and Twirp
client, respectively.  The index.ts file helps with module exports and provides
a thin wrapper which emits events for fetch start, finish, and failure.  The
files can be regenerated using:

    go generate ./autoroll/go/rpc