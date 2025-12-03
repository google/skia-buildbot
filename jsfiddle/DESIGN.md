## URLs

The URL structure of fiddle is:

    /canvaskit/<fiddlehash> - Direct link to a fiddle. (type CanvasKit)

The API to get just the code is:

    /_/code?type=canvaskit&hash=<fiddlehash>

which returns a response in the form:

    {
      "code": "let firstPath = CanvasKit....",
    }

To create a new fiddle, POST JSON to /\_/save of the form:

    {
      "code":"let firstPath = CanvasKit....",
      "type": "canvaskit",
    }

This returns JSON of the form:

    {
      "new_url": "/canvaskit/<fiddlehash>",
    }

## Storage

Fiddles are stored in Google Storage under gs://skia-jsfiddle/
For each fiddle we store the user's code at:

    gs://skia-jsfiddle/<type>/fiddle/<fiddlehash>/draw.js

The value "type" is "canvaskit", etc.
The value "fiddlehash" is the sha256 of the contents of draw.js (which does not
have line numbers in it).
