package main

import (
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/oauth2redirect"
)

func main() {
	sklog.Fatal(oauth2redirect.Main())
}
