Skia Debugger Asset Server
==========================

See /infra/debugger/README.md for a description of this app.

Running
=======

To run the server locally make sure you have Go installed and then run:

~~~~bash
go get go.skia.org/infra/debugger/...
cd $GOPATH/src/go.skia.org/infra/debugger
make run_server_local
~~~~

Make sure you have `$GOPATH/bin` added to your `PATH`.

This will spin up a local asset server on port 9000.

Make sure when you run the command-line debugger that it runs looking for
http://localhost:9000 and not https://debugger-assets.skia.org. I.e

    ./out/Release/skiaserve --source http://localhost:9000

