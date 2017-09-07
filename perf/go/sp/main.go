package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

func main() {
	common.Init()

	ctx := context.Background()
	client, err := datastore.NewClient(ctx, "google.com:skia-buildbots", option.WithCredentialsFile("service-account.json"))
	if err != nil {
		sklog.Fatal(err)
	}
	key := &datastore.Key{
		Kind:      "Shortcut",
		Namespace: "perftest",
	}
	type Shortcut struct {
		Keys []string
	}
	src := &Shortcut{
		Keys: []string{"foo", "bar"},
	}
	key, err = client.Put(ctx, key, src)
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Printf("Key: %d", key.ID)

	dst := &Shortcut{}
	if err := client.Get(ctx, key, dst); err != nil {
		sklog.Fatal(err)
	}
	fmt.Printf("Value: %v", *dst)
}
