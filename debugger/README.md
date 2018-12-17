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
    $ docker pull gcr.io/skia-public/skia-release:prod
    $ make release_ci
    $ make run_with_local_assets
~~~~

This builds the same docker image that runs in prod, including a copy of
skiaserve built against SwiftShader, so that GPU will work w/o needing
a physical GPU.
