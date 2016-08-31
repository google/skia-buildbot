luci-go: LUCI in Go: AppEngine server code
==========================================

Code layout
-----------

  * [/appengine/cmd/...](https://github.com/luci/luci-go/tree/master/appengine/cmd)
    contains the individual
    [AppEngine](https://cloud.google.com/appengine/docs/go/) servers, one per
    subdirectory.
  * [/appengine/internal/...](https://github.com/luci/luci-go/tree/master/appengine/internal)
    contains shared internal packages for use by other packages in this
    repository that are not meant to be used by other packages outside
    `/appengine/...`. See https://golang.org/s/go14internal for more details.
  * Anything else is APIs (reusable AppEngine-specific packages) that can be
    reused.
