package main

import (
	"context"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
)

/*
	Update metadata in GCE and on jumphosts.
*/

func main() {
	common.Init()
	ctx := context.Background()

	co, err := git.NewTempCheckout(ctx, "https://skia.googlesource.com/infra_metadata.git")
	if err != nil {
		sklog.Fatal(err)
	}

}
