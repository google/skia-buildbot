luci-go: LUCI in Go: server code
================================

Installing servers
------------------

    go get -u github.com/luci/luci-go/server/cmd/...


Code layout
-----------

  * [/server/cmd/...](https://github.com/luci/luci-go/tree/master/server/cmd)
    contains the individual server executables, one per subdirectory.
  * [/server/internal/...](https://github.com/luci/luci-go/tree/master/server/internal)
    contains shared internal packages for use by other packages in this
    repository that are not meant to be used by other packages outside
    `/server/...`. See https://golang.org/s/go14internal for more details.
  * Anything else is APIs (reusable packages) that can be reused; mostly to be
    reused by `/appengine/...`
