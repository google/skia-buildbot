package main

import (
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/authproxy"
)

func main() {
	sklog.Fatal(authproxy.Main())
}
