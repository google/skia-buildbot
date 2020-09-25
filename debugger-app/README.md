Skia Debugger Asset Server
==========================

A web-based tool for viewing and inspecting SKP and MSKP files (recorded SkPictures)

The debugger consists of one-page app that that loads a wasm module loosely based on canvaskit
the wasm module draws SKP file being inspected to the canvas in the center of the page.


Running locally
---------------

Production deployment
---------------------

Previous versions
-----------------

At the time of this writing, there is also `debugger-assets`, a polymer-based version of debugger
which also embeds the wasm module, and prior to that `debugger` which fetched images from a
backend server and did not involve wasm.

These are both planned to be deprecated and deleted early 2021.