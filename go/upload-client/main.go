package main

import (
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/upload"
)

func main() {
	common.Init()
	args := flag.Args()
	fileName := args[0]
	url := args[1]

	md5Hash, err := upload.UploadFile(fileName, url)
	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}
	sklog.Infof("Uploaded file: %s (%s)", fileName, md5Hash)
}
