luci-go: LUCI in Go: client code
================================

[![GoDoc](https://godoc.org/github.com/luci/luci-go/client?status.svg)](https://godoc.org/github.com/luci/luci-go/client)


Installing clients
------------------

    go get -u github.com/luci/luci-go/client/cmd/...


Code layout
-----------

  * [/client/cmd/...](https://github.com/luci/luci-go/tree/master/client/cmd)
    contains the individual executables, one per subdirectory.
  * [/client/internal/...](https://github.com/luci/luci-go/tree/master/client/internal)
    contains shared internal packages for use by other packages in this
    repository that are not meant to be used by other packages outside
    `/client/...`. See https://golang.org/s/go14internal for more details.
  * Anything else is APIs (reusable packages) that can be reused.
