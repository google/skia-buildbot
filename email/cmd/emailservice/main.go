package main

import (
	"go.skia.org/infra/email/go/emailservice"
	"go.skia.org/infra/go/sklog"
)

func main() {
	sklog.Fatal(emailservice.Main())
}
