Skia Debugger
=============

The Skia Debugger consists of two components, the C++ command-line application
that ingests SKPs and analyzes them, and the HTML/CSS/JS front-end to the
debugger that is loaded off of https://debugger.skia.org. This directory
contains the code for debugger.skia.org.

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
http://localhost:9000 and not https://debugger.skia.org.
