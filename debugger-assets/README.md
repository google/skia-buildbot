Skia Debugger Asset Server
==========================

See /infra/debugger/README.md for a description of this app.

Running
=======

The debugger-assets application serves up the HTML/CSS/JS that makes up
the debugger web UI. This is the place to make changes in the front-end
functionality of the debugger.

To run the server locally make sure you have Go installed and then run:

~~~~bash
    make run_server_local
~~~~

Make sure you have `$GOPATH/bin` added to your `PATH`.

This will spin up a local asset server on port 9000.

At the same time you need a copy of `skiaserver` running that does the actual
debugging work. You can build this as part of building the Skia library. Make
sure when you run the command-line debugger that it runs looking for
http://localhost:9000 and not https://debugger-assets.skia.org. I.e:

    ./out/Release/skiaserve --source http://localhost:9000
