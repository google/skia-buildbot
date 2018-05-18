Skia Debugger
=============

The Skia Debugger consists of several components:
  - skiaserve - The C++ command-line application that ingests SKPs and analyzes them.
  - debugger-assets - The server that provides the HTML/CSS/JS for skiaserve.
  - debugger - The server that sits at debugger.skia.org and proxies requests
    to running skiaserve instances.

See `DESIGN.md` for more details.


Running
=======

To run the server locally make sure you have Go installed and then run:

~~~~bash
go get go.skia.org/infra/debugger/...
cd $GOPATH/src/go.skia.org/infra/debugger
make run_server_local
~~~~

Make sure you have `$GOPATH/bin` added to your `PATH`.

This will spin up a local server on port 9000.

Make sure when you run the command-line debugger that it runs looking for
http://localhost:9000 and not https://debugger.skia.org. I.e

    ./out/Release/skiaserve --source http://localhost:9000

