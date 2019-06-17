Skia Debugger Asset Server
==========================

See /infra/debugger/README.md for a description of this app.

Running
-------

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

    bin/gn gen out/Debug
    ninja -C out/Debug skiaserve
    ./out/Debug/skiaserve --source http://localhost:9000

WASM Debugger
=============

There is also a version of the debugger that uses a wasm module instead of the skiaserve backend.
It is always served at http://debugger-assets.skia.org/res/v2.html.
When debugger-assets is run with --v2_at_root it is also served at /.
A version of debugger-assets running this flag is served at debugger.skia.org, as this version of
the debugger has become the primary one as of June 2019.

This application is in res/imp/wasm-app.html and its wasm code is in
experimental/wasm-skp-debugger in the skia repo.

Running WASM debugger locally
---------------

First build the wasm module and its associated Javascript in the Skia repository.

    cd {skia}/experimental/wasm-skp-debugger
    make debug
    make move-assets

This will copy the two outputs from `out/debugger_wasm` to
`~/go/src/go.skia.org/infra/debugger-assets/res`

You could also `make release` instead to test the closure compilation and externs.

Since this version requires no backend, the custom app element is simply instantiated in
res/imp/wasm-app-demo.html which can be loaded in the browser. It will load debugger.wasm and
debugger.js which were built in the previous step. It is necessary to serve debugger.wasm from
an http server rather than opening the file directly because wasm-loading code requires the mime
type to be correct. To start this server:

    cd ~/go/src/go.skia.org/infra/debugger-assets
    make run_server_local_wasm

then visit <http://localhost:9000/res/imp/wasm-app-demo.html>

Running WASM debugger within docker
---------------------

The wasm debugger can also be tested while debugger-assets runs within Docker. In this
configuration it will vulcanize and minify the elements and javascript, and pull debugger.wasm
and debugger.js from `gcr.io/skia-public/skia-wasm-release:prod` instead of your local filesystem.

    SKIP_UPLOAD=1 make release
    docker run --expose=8000 -p 8000:8000 debugger-assets:latest --v2_at_root

then visit <http://localhost:8000/>