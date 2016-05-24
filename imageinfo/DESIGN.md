imageinfo
=========

Web application to show the color gamut of a given image. I.e. an easier to use
version of the following:

    ./bin/sync-and-gyp
    ninja -C  out/Release visualize\_color\_gamut
    ./out/Release/visualize\_color\_gamut --input foo.png --output bar.png

A user either uploads a PNG, or provides a public URL to a PNG, and the
application shows information about the color gamut of the image.

Uploaded images will be stored in Google Storage, filenames being the MD5 hash
of the image contents. The upload function only works if you are logged in.

Users will also want to see the numbers that visualize\_color\_gamut emits,
possibly along with the response curves.


URL Structure
-------------

    /info?hash=

Where `hash` is the MD5 hash of the uploaded image. Anyone can visit this URL,
but only logged in users can upload images. This is not currently implemented.

Providing a link to a public image on the web always works even if not logged
in.

    /info?url=

For each call we always download the image and pass it through
`visualize\_color\_gamut`.

Operation
---------

Requests for an analysis of an image go to the `/info` endpoint. Regardless of
whether the `url` or `hash` query parameters are supplied, the application
downloads the appropriate image and then passes it through
`visualize\_color\_gamut`. The resulting output image is kept in a local LRU
cache, so that it can later be served to the requesting web page.


Building
--------

The `visualize\_color\_gamut` is built periodically at LKGR on the server.

By default $WORK\_ROOT is /mnt/pd0, but can be another directory when running
locally and not using systemd-nspawn.

Skia is checked out into $WORK\_ROOT/versions/<githash>, and the target
`visualize\_color\_gamut` is built using ninja.

Good builds are recorded in $WORK\_ROOT/goodbuilds.txt, which is just a text
file of good builds in the order they are done, that is, new good builds are
appended to the end of the file.

A small number of recent good builds are kept, but only the most recent one is
used.

Drive
-----

An attached disk will reside at /mnt/pd0 and will be populated as:

     /mnt/pd0/imageinfo     - $WORK_ROOT
     /mnt/pd0/imageinfo/tmp - Dir where tmp dirs are created for the runs
                              of `visualize\_color\_gamut`.
     /mnt/pd0/depot_tools
