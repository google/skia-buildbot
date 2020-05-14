Status Elements
=================

Before being able to run the demo pages, run the
```
make web
```
in the infra/status directory.

To run the demo pages, navigate to this directory and run
```
make run
```

This will download the necessary dependencies, download the demo data from
[Google Storage](https://console.cloud.google.com/storage/browser/skia-infra-testdata/status-demo/?project=google.com:skia-buildbots)
and set up a local server for debugging on http://localhost:8080.


The general idea for many of these elements is to have a *-data element that fetches the
data needed for rendering, parses and formats it, and then makes it available via data bindings.
There are then UI elements that takes the data and renders it.  The UI elements may have
user-operable widgets, the output of which are exposed for databinding as well.  This helps break
apart the components and makes it easier to, for example, change from using Polymer to using D3 for
a component of the visualization.

For performance reasons, the drawing of the main table on status (commits-table-sk) is done with D3.
An excellent primer on D3 is [Interactive Data Visualization for the Web](http://chimera.labs.oreilly.com/books/1230000000345/index.html) by Scott Murray.

The elements were previously written in Polymer 0.5.
Many of these were deleted after commit [9b8bb9c04](https://skia.googlesource.com/buildbot/+show/9b8bb9c044470b039fb5ee8e82cb106d16492829).
