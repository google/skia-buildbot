URLs
----

The URL structure of fiddle is:

    /pathkit/cbb8dee39e9f1576cd97c2d504db8eee - Direct link to a fiddle. (type PathKit)
    /canvaskit/cbb8dee39e9f1576cd97c2d504db8eee - Direct link to a fiddle. (type PathKit)

To create a new fiddle, POST JSON to /\_/save of the form:

    {
      "code":"let firstPath = PathKit....",
      "type": "pathkit",
    }

This returns JSON of the form:

    {
      "new_url": "/pathkit/<fiddlehash>",
    }

Storage
-------

Fiddles are stored in Google Storage under gs://skia-jsfiddle/
For each fiddle we store the user's code at:

    gs://skia-jsfiddle/<type>/fiddle/<fiddlehash>/draw.js

The value "type" is "pathkit", "canvaskit", etc.
The value "fiddlehash" is the sha256 of the contents of draw.js (which does not
have line numbers in it).