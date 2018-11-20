package main

import (
	"context"

	"cloud.google.com/go/firestore"
	apiv1beta1 "cloud.google.com/go/firestore/apiv1beta1"
	"go.skia.org/infra/go/auth"
	"google.golang.org/api/option"
)

func main() {
	ts, err := auth.NewDefaultTokenSource(true)
	if err != nil {
		panic(err.Error())
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "skia-firestore", option.WithTokenSource(ts))
	if err != nil {
		panic(err.Error())
	}

	coll := client.Collection("fake-collection")
	ref := coll.NewDoc()
	if _, err = ref.Create(ctx, map[string]string{
		"hello": "world",
	}); err != nil {
		panic(err.Error())
	}
	apiv1beta1.MockCommitAborted = true
	if err := client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(ref)
		if err != nil {
			return err
		}
		data := snap.Data()
		data["label"] = "my-label"
		return tx.Set(ref, data)
	}); err != nil {
		panic(err.Error())
	}
}
