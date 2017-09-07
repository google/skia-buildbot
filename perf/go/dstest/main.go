package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/ds"
)

type Shortcut struct {
	Keys []string
}

func main() {
	common.Init()
	if err := ds.Init("google.com:skia-buildbots"); err != nil {
		sklog.Fatal(err)
	}

	key := &datastore.Key{
		Kind:      "Shortcut",
		Namespace: "perftest",
	}
	src := &Shortcut{
		Keys: []string{"foo", "bar"},
	}
	var err error
	ctx := context.Background()
	key, err = ds.DS.Put(ctx, key, src)
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Printf("Key: %d\n", key.ID)

	dst := &Shortcut{}
	if err := ds.DS.Get(ctx, key, dst); err != nil {
		sklog.Fatal(err)
	}
	fmt.Printf("Keys: %v\n", *dst)
}
