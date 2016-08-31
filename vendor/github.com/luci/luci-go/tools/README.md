luci-go: LUCI in Go: LUCI support tool code
===========================================

Code layout
-----------

  * [/tools/cmd/...](https://github.com/luci/luci-go/tree/master/tools/cmd)
    contains the individual tools.
  * [/tools/internal/...](https://github.com/luci/luci-go/tree/master/tools/internal)
    contains shared internal packages for use by other packages in this
    repository that are not meant to be used by other packages outside
    `/tools/...`. See https://golang.org/s/go14internal for more details.
