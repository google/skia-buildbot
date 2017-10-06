package main

import (
	"flag"
	"net/http"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/upload"
)

func main() {
	common.Init()
	addr := flag.Args()[0]

	http.HandleFunc("/upload", upload.UploadHandler)
	sklog.Infof("Listening to: http://localhost" + addr)
	http.ListenAndServe(addr, nil)
}
